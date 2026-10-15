package util

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProcessPropertiesFile(t *testing.T) {
	data := []byte(`item1.key1: value1
item1.key2: value2
item1.key3: value3.1
 value3.2
 value3.3

item2.key1: value1
item2.key2: value2
item2.key3:
 value3.1
 value3.2`)

	expected := []map[string]string{
		{
			"item1.key1": "value1",
			"item1.key2": "value2",
			"item1.key3": "value3.1\nvalue3.2\nvalue3.3",
		},
		{
			"item2.key1": "value1",
			"item2.key2": "value2",
			"item2.key3": "value3.1\nvalue3.2",
		},
	}

	actual, err := ProcessPropertiesFile(bytes.NewReader(data))
	assert.EqualValues(t, expected, actual)
	assert.Nil(t, err)

	badData := []byte(`item1.key1: value1
bad line`)

	actual, err = ProcessPropertiesFile(bytes.NewReader(badData))
	assert.Error(t, err)
}
