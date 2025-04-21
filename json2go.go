package json2go

import (
	"fmt"
	"sort"
	"strings"
)

const (
	TypeString    = "string"
	TypeInt       = "int"
	TypeFloat     = "float64"
	TypeBool      = "bool"
	TypeMap       = "map"
	TypeArray     = "array"
	TypeInterface = "interface{}"
	ArrayKey      = "0"
)

type StructType struct {
	Parent string
	Name   string
	Type   string
	Tag    string
	Fields map[string]*StructType
}

func (s *StructType) GoType(isAnonymous bool) string {
	if isAnonymous && s.Type == TypeMap {
		// 如果是匿名结构体，返回结构体定义的内容
		var sb strings.Builder
		sb.WriteString("struct {\n")
		keys := make([]string, 0, len(s.Fields))
		for k := range s.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := s.Fields[k]
			var t = ""
			if v.Type == TypeMap {
				t = "*"
			}
			sb.WriteString(fmt.Sprintf("    %s %s%s `json:\"%s\"`\n", k, t, v.GoType(isAnonymous), strings.ToLower(k)))
		}
		sb.WriteString("}")
		return sb.String()
	}

	switch s.Type {
	case TypeMap:
		return s.Parent + s.Name
	case TypeArray:
		return "[]" + s.Fields["0"].GoType(isAnonymous)
	default:
		return s.Type
	}
}

func (s *StructType) GenGoStructs(isAnonymous bool) string {
	if s.Parent == "" && s.Type == TypeArray {
		return s.Fields[ArrayKey].GenGoStructs(isAnonymous)
	}

	var structs []string
	visited := make(map[string]bool)

	var collect func(st *StructType)
	collect = func(st *StructType) {
		if st.Type != TypeMap {
			return
		}
		key := st.Parent + st.Name
		if visited[key] {
			return
		}
		visited[key] = true
		if !isAnonymous {
			for _, f := range st.Fields {
				collect(f)
			}
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("type %s struct {\n", key))

		keys := make([]string, 0, len(st.Fields))
		for k := range st.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := st.Fields[k]

			var tmp = ""
			if v.Type == TypeMap {
				tmp = "*"
			}

			sb.WriteString(fmt.Sprintf("    %s %s%s `json:\"%s\"`\n", k, tmp, v.GoType(isAnonymous), strings.ToLower(k)))
		}
		sb.WriteString("}\n")
		structs = append(structs, sb.String())
	}
	collect(s)
	return strings.Join(structs, "\n")
}

// mergeStruct 合并两个 map 类型结构体的字段，如果字段类型不同则跳过合并
func mergeStruct(dest, src *StructType) {
	if dest.Type != src.Type {
		panic(fmt.Errorf("type error at %s", dest.Parent+dest.Name))
	}

	if dest.Type != TypeMap || src.Type != TypeMap {
		return
	}

	for k, v := range src.Fields {
		if existing, ok := dest.Fields[k]; ok {
			// 只合并同类型字段
			if existing.Type == v.Type {
				mergeStruct(existing, v)
			} else {
				// 处理不同类型字段的警告
				fmt.Printf("Warning: Field %s has different types, skipping merge\n", k)
			}
		} else {
			dest.Fields[k] = v
		}
	}
}

// toExported 转换为 Go 公共字段名称规范
func toExported(s string) string {
	if s == "" {
		return ""
	}
	// 处理包含下划线的字段名
	s = strings.ReplaceAll(s, "_", "")
	return strings.ToUpper(s[:1]) + s[1:]
}

func DetectType(val any, name string, parent string) *StructType {
	return detectType(val, name, parent)
}

// detectType 根据输入的值推断字段的类型，并生成相应的 StructType
func detectType(val any, name string, parent string) *StructType {
	st := &StructType{
		Name:   name,
		Parent: parent,
		Fields: make(map[string]*StructType),
	}

	switch v := val.(type) {
	case nil:
		st.Type = "interface{}"
	case bool:
		st.Type = TypeBool
	case float64:
		if float64(int64(v)) == v {
			st.Type = TypeInt
		} else {
			st.Type = TypeFloat
		}
	case string:
		st.Type = TypeString
	case []any:
		st.Type = TypeArray
		if len(v) > 0 {
			// 推断数组元素类型
			elem := detectType(v[0], "", parent+name)
			st.Fields[ArrayKey] = elem
			for _, item := range v[1:] {
				elem := detectType(item, "", parent+name)
				mergeStruct(st.Fields[ArrayKey], elem)
			}
		} else {
			st.Fields[ArrayKey] = &StructType{Type: "interface{}"}
		}
	case map[string]any:
		st.Type = "map"
		for key, value := range v {
			goField := toExported(key)
			field := detectType(value, goField, parent+name)
			st.Fields[goField] = field
		}
	default:
		st.Type = "interface{}"
	}
	return st
}
