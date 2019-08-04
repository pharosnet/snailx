package snailx

import (
	"encoding/json"
	"reflect"
)

var emptyJsonObjectType = reflect.TypeOf(&JsonObject{})

type JsonObject struct {
	data map[string]interface{}
}

func (j *JsonObject) Encode() []byte {
	if j.data == nil {
		return []byte("{}")
	}
	if p, err := json.Marshal(j.data); err == nil {
		return p
	}
	return []byte("{}")
}

func (j *JsonObject) Decode(p []byte) error {
	j.data = make(map[string]interface{})
	return json.Unmarshal(p, &j.data)
}

func (j *JsonObject) Put(key string, val interface{}) {
	if j.data == nil {
		j.data = make(map[string]interface{})
	}
	j.data[key] = val
}

func (j *JsonObject) Get(key string) interface{} {
	if j.data == nil {
		return nil
	}
	if v, has := j.data[key]; has {
		return v
	}
	return nil
}

func (j *JsonObject) GetString(key string) string {
	if j.data == nil {
		return ""
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(string); ok {
			return value
		}
		return ""
	}
	return ""
}

func (j *JsonObject) GetStringWithDefault(key string, def string) string {
	if j.data == nil {
		return def
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(string); ok && value != "" {
			return value
		}
		return def
	}
	return def
}

func (j *JsonObject) GetInt(key string) int {
	if j.data == nil {
		return 0
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(int); ok {
			return value
		}
		return 0
	}
	return 0
}

func (j *JsonObject) GetIntWithDefault(key string, def int) int {
	if j.data == nil {
		return def
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(int); ok && value != 0 {
			return value
		}
		return def
	}
	return def
}

func (j *JsonObject) GetFloat(key string) float32 {
	if j.data == nil {
		return 0
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(float32); ok {
			return value
		}
		return 0
	}
	return 0
}

func (j *JsonObject) GetFloatWithDefault(key string, def float32) float32 {
	if j.data == nil {
		return def
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(float32); ok && value != 0 {
			return value
		}
		return def
	}
	return def
}

func (j *JsonObject) GetBool(key string) bool {
	if j.data == nil {
		return false
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(bool); ok {
			return value
		}
		return false
	}
	return false
}

func (j *JsonObject) GetBoolWithDefault(key string, def bool) bool {
	if j.data == nil {
		return def
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(bool); ok && value == false {
			return value
		}
		return def
	}
	return def
}

func (j *JsonObject) GetJsonObject(key string) *JsonObject {
	if j.data == nil {
		return nil
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(map[string]interface{}); ok {
			return &JsonObject{data: value}
		}
		return nil
	}
	return nil
}

func (j *JsonObject) GetJsonObjectWithDefault(key string, def *JsonObject) *JsonObject {
	if j.data == nil {
		return def
	}
	if v, has := j.data[key]; has {
		if value, ok := v.(map[string]interface{}); ok && value != nil {
			return &JsonObject{data: value}
		}
		return def
	}
	return def
}

type JsonArray struct {
	data []interface{}
}

func (j *JsonArray) Encode() []byte {
	if j.data == nil {
		return []byte("[]")
	}
	if p, err := json.Marshal(j.data); err == nil {
		return p
	}
	return []byte("[]")
}

func (j *JsonArray) Decode(p []byte) error {
	j.data = make([]interface{}, 0, 1)
	return json.Unmarshal(p, &j.data)
}

func (j *JsonArray) Add(key string, v interface{}) {
	if j.data == nil {
		j.data = make([]interface{}, 0, 1)
	}
	if v == nil {
		return
	}
	if value, ok := v.(*JsonObject); ok && value != nil && value.data != nil && len(value.data) > 0 {
		j.data = append(j.data, value.data)
		return
	}
	if value, ok := v.(JsonObject); ok && value.data != nil && len(value.data) > 0 {
		j.data = append(j.data, value.data)
		return
	}
	if reflect.TypeOf(v).Kind() == reflect.Struct || reflect.TypeOf(v).Kind() == reflect.Ptr {
		p, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		data := make(map[string]interface{})
		err = json.Unmarshal(p, data)
		if err != nil {
			panic(err)
		}
		j.data = append(j.data, data)
		return
	}
	j.data = append(j.data, v)
	return
}
