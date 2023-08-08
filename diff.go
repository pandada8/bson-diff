package bsondiff

import (
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// Diff generate
func Diff(a, b interface{}, ignoredField []string) (ret bson.M, err error) {
	ctx := CompareContext{
		Ignore: ignoredField,
		Left:   interfaceToDocument(a),
		Right:  interfaceToDocument(b),
	}
	payload := ctx.Compare()
	retDoc, err := payload.MarshalBSON()
	if err != nil {
		return
	}

	err = bson.Unmarshal(retDoc, &ret)
	return
}

func diffBSONDocument(a, b, d bsoncore.Document) (err error) {
	return nil
}

func interfaceToDocument(target interface{}) (ret bsoncore.Document) {
	bsonBytes, err := bson.Marshal(target)
	if err == nil {
		ret = bsoncore.Document(bsonBytes)
	}
	return
}

type CompareContext struct {
	Ignore       []string
	Left         bsoncore.Document
	Right        bsoncore.Document
	Stack        []CompareStackFrame
	CurrentFrame CompareStackFrame
}

type CompareStackFrame struct {
	Left   []bsoncore.Element
	Right  []bsoncore.Element
	Prefix []string
}

func (f *CompareStackFrame) GetKeyArr(name string) []string {
	return append(f.Prefix, name)
}
func (f *CompareStackFrame) GetKey(name string) string {
	return strings.Join(f.GetKeyArr(name), ".")
}

func NewCompareStackFrame(prefix []string, left, right bsoncore.Document) CompareStackFrame {
	lefts, _ := left.Elements()
	rights, _ := right.Elements()
	return CompareStackFrame{
		Prefix: prefix,
		Left:   lefts,
		Right:  rights,
	}
}

func (c *CompareContext) IsIgnored(key string) bool {
	for _, ignore := range c.Ignore {
		if ignore == key {
			return true
		}
	}
	return false
}

func (c *CompareContext) Compare() (ret bsonx.Doc) {
	// init CompareStack
	c.Stack = append(c.Stack, NewCompareStackFrame([]string{}, c.Left, c.Right))
	for len(c.Stack) != 0 {
		// pop first frame
		c.CurrentFrame, c.Stack = c.Stack[0], c.Stack[1:]
		cacheLeft := make(map[string]bsoncore.Value)
		cacheRight := make(map[string]bsoncore.Value)
		rightAccessed := make(map[string]bool)

		// populate the cache
		for _, elem := range c.CurrentFrame.Left {
			cacheLeft[elem.Key()] = elem.Value()
		}
		for _, elem := range c.CurrentFrame.Right {
			cacheRight[elem.Key()] = elem.Value()
			rightAccessed[elem.Key()] = false
		}

		// start comparing
		for key, valueLeft := range cacheLeft {
			valueRight, ok := cacheRight[key]
			if !ok {
				// the key is deleted on right
				if c.IsIgnored(key) {
					continue
				}
				// unset: {"$prefix.$key": true}
				ret = setValue(ret, bsonx.Boolean(true), "$unset", c.CurrentFrame.GetKey(key))
				continue
			}
			rightAccessed[key] = true
			// check if right and left is same
			if valueLeft.Equal(valueRight) {
				continue
			}
			// if key is in ignore list, ignore it
			if c.IsIgnored(key) {
				continue
			}
			// is both document ?
			if valueLeft.Type == valueRight.Type && bsontype.EmbeddedDocument == valueRight.Type {
				// push to stack, compare internal values
				c.Stack = append(c.Stack, NewCompareStackFrame(append(c.CurrentFrame.Prefix, key), valueLeft.Document(), valueRight.Document()))
				continue
			}
			// is both array ?
			if valueLeft.Type == valueRight.Type && bsontype.Array == valueRight.Type {
				// TODO: handle array

			}
			// FIXME: check inside array
			// so now left != right
			ret = setValue(ret, valueToVal(valueRight), "$set", c.CurrentFrame.GetKey(key))
		}

		for key, accessed := range rightAccessed {
			if accessed == false {
				// the key is newly inserted on right
				if c.IsIgnored(key) {
					continue
				}
				value := cacheRight[key]
				ret = setValue(ret, valueToVal(value), "$set", c.CurrentFrame.GetKey(key))
			}
		}
	}
	return
}

func debugDoc(doc interface{}) {
	switch docData := doc.(type) {
	case bsoncore.Document:
		payload, _ := bson.MarshalExtJSON(docData, true, false)
		fmt.Println(string(payload))
	case bsonx.Doc:
		payload, _ := bson.MarshalExtJSON(docData, true, false)
		fmt.Println(string(payload))
	default:
		fmt.Println(doc)
	}
}

func valueToVal(v bsoncore.Value) (ret bsonx.Val) {
	ret = bsonx.Val{}
	_ = ret.UnmarshalBSONValue(v.Type, v.Data)
	return
}

// TODO: expose this function
func setValue(doc bsonx.Doc, value bsonx.Val, keys ...string) bsonx.Doc {
	if len(keys) == 0 {
		return doc
	}

	stack := []bsonx.Doc{doc}

	// walk down keys and collect each level document
	var last bsonx.Doc
	for n, key := range keys[:len(keys)-1] {
		last = stack[n]
		val, err := last.LookupErr(key)
		if err != nil {
			// if not exist, then create new one
			stack = append(stack, bsonx.Doc{})
		} else {
			targetDoc, ok := val.DocumentOK()
			if !ok {
				// if is not document, create new one to replace
				stack = append(stack, bsonx.Doc{})
			} else {
				stack = append(stack, targetDoc)
			}
		}
	}
	// set actual payload on deepest level object
	stack[len(stack)-1] = stack[len(stack)-1].Set(keys[len(keys)-1], value)
	if len(keys) > 1 {
		// travel back to root
		for i := len(stack) - 2; i >= 0; i-- {
			// update each document with new value
			stack[i] = stack[i].Set(keys[i], bsonx.Document(stack[i+1]))
		}
	}
	// return root
	return stack[0]
}
