package scim

import (
	"strconv"
	"strings"
)

func ParseScimGroups(fields []map[string]any) (groups []string) {
	for _, field := range fields {
		var v any
		var ok bool
		if v, ok = field["value"]; ok {
			if v == nil {
				continue
			}
			switch vt := v.(type) {
			case []any:
				for _, v = range vt {
					var group string
					if group, ok = v.(string); ok {
						groups = append(groups, group)
					}
				}
			case string:
				groups = append(groups, vt)
			}
		}
	}
	return
}

func toBoolean(intf any) (result bool, ok bool) {
	if intf == nil {
		return
	}
	var supportedValue any
	switch fv := intf.(type) {
	case bool, string:
		supportedValue = fv
	case []any:
		if len(fv) > 0 {
			switch fv[0].(type) {
			case bool, string:
				supportedValue = fv[0]
			}
		}
	}
	if supportedValue != nil {
		switch fv := supportedValue.(type) {
		case bool:
			result = fv
			ok = true
		case string:
			switch strings.ToLower(fv) {
			case "1", "true", "ok":
				result = true
				ok = true
			case "0", "false":
				result = false
				ok = true
			}
		}
	}
	return
}

func toString(intf any) (result string, ok bool) {
	if intf == nil {
		return
	}
	result, ok = intf.(string)
	return
}

func toInt64(intf interface{}) (result int64, ok bool) {
	if intf == nil {
		return
	}
	ok = true
	switch iv := intf.(type) {
	case int:
		result = int64(iv)
	case uint:
		result = int64(iv)
	case int8:
		result = int64(iv)
	case uint8:
		result = int64(iv)
	case int16:
		result = int64(iv)
	case uint16:
		result = int64(iv)
	case int32:
		result = int64(iv)
	case uint32:
		result = int64(iv)
	case int64:
		result = iv
	case uint64:
		result = int64(iv)
	case float32:
		result = int64(iv)
	case float64:
		result = int64(iv)
	case string:
		if irv, err := strconv.Atoi(iv); err == nil {
			result = int64(irv)
		} else {
			ok = false
		}
	default:
		ok = false
	}
	return
}

type Set[K comparable] map[K]struct{}

func NewSet[K comparable]() Set[K] {
	return make(Set[K])
}
func MakeSet[K comparable](keys []K) Set[K] {
	var ns = NewSet[K]()
	for _, k := range keys {
		ns.Add(k)
	}
	return ns
}
func (s Set[K]) Enumerate(cb func(K) bool) {
	for k := range s {
		if !cb(k) {
			break
		}
	}
}
func (s Set[K]) Has(key K) (ok bool) {
	_, ok = s[key]
	return
}
func (s Set[K]) Add(key K) {
	s[key] = struct{}{}
}
func (s Set[K]) Delete(key K) {
	delete(s, key)
}
func (s Set[K]) ToArray() (result []K) {
	for k := range s {
		result = append(result, k)
	}
	return
}
func (s Set[K]) Copy() Set[K] {
	var ns = NewSet[K]()
	for k := range s {
		ns.Add(k)
	}
	return ns
}
func (s Set[K]) EqualTo(other Set[K]) (result bool) {
	if len(s) == len(other) {
		var ok bool
		for k := range s {
			if _, ok = other[k]; !ok {
				return
			}
		}
	}
	return true
}
func (s Set[K]) Union(other []K) {
	for _, k := range other {
		s.Add(k)
	}
}
func (s Set[K]) Intersect(other []K) {
	for _, k := range other {
		if !s.Has(k) {
			delete(s, k)
		}
	}
}
func (s Set[K]) Difference(other []K) {
	for _, k := range other {
		if s.Has(k) {
			delete(s, k)
		}
	}
}
