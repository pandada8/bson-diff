package bsondiff

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func jsonToBson(payload string) (ret bsoncore.Document) {
	var data bson.Raw
	err := bson.UnmarshalExtJSON([]byte(payload), true, &data)
	if err != nil {
		fmt.Println(payload)
		panic("fail to decode json")
	}
	ret = bsoncore.Document(data)
	return
}

func docToString(doc interface{}) string {
	switch docData := doc.(type) {
	case bsonx.Doc:
		payload, _ := bson.MarshalExtJSON(docData, true, false)
		return string(payload)
	case bson.M:
		payload, _ := bson.MarshalExtJSON(docData, true, false)
		return string(payload)
	}
	return ""
}

func Test_SetValue(t *testing.T) {
	var doc bsonx.Doc
	doc = setValue(bsonx.Doc{}, bsonx.String("b"), "a")
	assert.Equal(t, `{"a":"b"}`, docToString(doc), "should be equal")
	doc = setValue(bsonx.Doc{}, bsonx.String("c"), "a", "b")
	assert.Equal(t, `{"a":{"b":"c"}}`, docToString(doc), "should be equal")
	doc = setValue(bsonx.Doc{}, bsonx.String("g"), "a", "b", "c", "d", "e", "f")
	assert.Equal(t, `{"a":{"b":{"c":{"d":{"e":{"f":"g"}}}}}}`, docToString(doc), "should be equal")
}

func Test_CompareValue(t *testing.T) {
	diff, err := Diff(bson.M{"b": "c"}, bson.M{"a": "c"}, nil)
	assert.Equal(t, `{"$unset":{"b":true},"$set":{"a":"c"}}`, docToString(diff), "should be equal")
	assert.NoError(t, err, "should be no error")
}

func Test_Nested(t *testing.T) {
	diff, err := Diff(bson.M{"a": bson.M{"b": "c"}}, bson.M{"a": bson.M{"e": "d"}}, nil)
	assert.Equal(t, `{"$unset":{"a.b":true},"$set":{"a.e":"d"}}`, docToString(diff), "should be equal")
	assert.NoError(t, err, "should be no error")
}
