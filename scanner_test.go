package structscanner_test

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	ss "github.com/vingarcia/structscanner"
	tt "github.com/vingarcia/structscanner/internal/testtools"
)

func TestDecode(t *testing.T) {
	t.Run("should parse a single tag with a hardcoded value", func(t *testing.T) {
		decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
			return "fake-value-for-string", nil
		})

		var output struct {
			Attr1 string `env:"attr1"`
		}
		err := ss.Decode(&output, decoder)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, output.Attr1, "fake-value-for-string")
	})

	t.Run("should ignore attributes if the function returns a nil value", func(t *testing.T) {
		decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
			envTag := field.Tags["env"]
			if envTag == "" {
				return nil, nil
			}

			return "fake-value-for-string", nil
		})

		var output struct {
			Attr1 string `env:"attr1"`
			Attr2 string `someothertag:"attr2"`
		}
		output.Attr2 = "placeholder"
		err := ss.Decode(&output, decoder)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, output.Attr1, "fake-value-for-string")
		tt.AssertEqual(t, output.Attr2, "placeholder")
	})

	t.Run("should be able to fill multiple attributes", func(t *testing.T) {
		decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
			v := map[string]string{
				"f1": "v1",
				"f2": "v2",
				"f3": "v3",
			}[field.Tags["map"]]

			return v, nil
		})

		var output struct {
			Attr1 string `map:"f1"`
			Attr2 string `map:"f2"`
			Attr3 string `map:"f3"`
		}
		err := ss.Decode(&output, decoder)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, output.Attr1, "v1")
		tt.AssertEqual(t, output.Attr2, "v2")
		tt.AssertEqual(t, output.Attr3, "v3")
	})

	t.Run("should ignore private fields", func(t *testing.T) {
		decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
			return "fake-value-for-string", nil
		})

		var output struct {
			Attr1 string `env:"attr1"`
			attr2 string `env:"attr2"`
		}
		err := ss.Decode(&output, decoder)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, output.Attr1, "fake-value-for-string")
		tt.AssertEqual(t, output.attr2, "")
	})

	t.Run("nested structs", func(t *testing.T) {
		t.Run("should parse fields recursively if a decoder is returned", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				if field.Kind == reflect.Struct {
					return ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
						return 42, nil
					}), nil
				}

				return 64, nil
			})

			var output struct {
				Attr1       int `env:"attr1"`
				OtherStruct struct {
					Attr2 int `env:"attr1"`
				}
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Attr1, 64)
			tt.AssertEqual(t, output.OtherStruct.Attr2, 42)
		})

		t.Run("should parse fields recursively even for nil pointers to struct", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				if field.Kind == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct {
					return ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
						return 42, nil
					}), nil
				}

				return 64, nil
			})

			var output struct {
				Attr1       int `env:"attr1"`
				OtherStruct *struct {
					Attr2 int `env:"attr2"`
				}
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Attr1, 64)
			tt.AssertEqual(t, output.OtherStruct.Attr2, 42)
		})

		t.Run("should report error correctly for invalid nested values", func(t *testing.T) {
			tests := []struct {
				desc               string
				targetStruct       interface{}
				expectErrToContain []string
			}{
				{
					desc: "not a struct",
					targetStruct: &struct {
						NotAStruct int `env:"attr1"`
					}{},
					expectErrToContain: []string{"NotAStruct", "can only get struct info from structs", "int"},
				},
				{
					desc: "pointer to not a struct",
					targetStruct: &struct {
						NotAStruct *int `env:"attr1"`
					}{},
					expectErrToContain: []string{"NotAStruct", "can only get struct info from structs", "*int"},
				},
			}
			for _, test := range tests {
				t.Run(test.desc, func(t *testing.T) {
					decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
						// Some tag decoder:
						return ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
							return 42, nil
						}), nil
					})

					err := ss.Decode(test.targetStruct, decoder)
					tt.AssertErrContains(t, err, test.expectErrToContain...)
				})
			}
		})
	})

	t.Run("nested slices", func(t *testing.T) {
		t.Run("should convert each item of a slice", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return []interface{}{1, 2, 3}, nil
			})

			var output struct {
				Slice []int `map:"slice"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Slice, []int{1, 2, 3})
		})

		t.Run("should convert each item of a slice even with different types", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return []interface{}{1, 2, 3}, nil
			})

			var output struct {
				Slice []float64 `map:"slice"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Slice, []float64{1.0, 2.0, 3.0})
		})

		t.Run("should work with slices of pointers", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return []*int{
					intPtr(1),
					intPtr(2),
					intPtr(3),
				}, nil
			})

			var output struct {
				Slice []int `map:"slice"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Slice, []int{1, 2, 3})
		})

		t.Run("should work with slices of pointers or different types", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return []*int{
					intPtr(1),
					intPtr(2),
					intPtr(3),
				}, nil
			})

			var output struct {
				Slice []float64 `map:"slice"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Slice, []float64{1.0, 2.0, 3.0})
		})

		t.Run("should work with pointers to slices", func(t *testing.T) {
			t.Run("source pointer target non-pointer", func(t *testing.T) {
				decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
					return &[]int{1, 2, 3}, nil
				})

				var output struct {
					Slice []int `map:"slice"`
				}
				err := ss.Decode(&output, decoder)
				tt.AssertNoErr(t, err)
				tt.AssertEqual(t, output.Slice, []int{1, 2, 3})
			})

			t.Run("source non-pointer target pointer", func(t *testing.T) {
				decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
					return []int{1, 2, 3}, nil
				})

				var output struct {
					Slice *[]int `map:"slice"`
				}
				err := ss.Decode(&output, decoder)
				tt.AssertNoErr(t, err)
				tt.AssertEqual(t, output.Slice, &[]int{1, 2, 3})
			})
		})
	})

	t.Run("should convert types correctly", func(t *testing.T) {
		t.Run("should convert different types of integers", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return uint64(10), nil
			})

			var output struct {
				Attr1 int `env:"attr1"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Attr1, 10)
		})

		t.Run("should convert from ptr to non ptr", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				i := 64
				return &i, nil
			})

			var output struct {
				Attr1 int `env:"attr1"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Attr1, 64)
		})

		t.Run("should convert from ptr to non ptr", func(t *testing.T) {
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return 64, nil
			})

			var output struct {
				Attr1 *int `env:"attr1"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertNotEqual(t, output.Attr1, nil)
			tt.AssertEqual(t, *output.Attr1, 64)
		})

		t.Run("should work with structs", func(t *testing.T) {
			type Foo struct {
				Name string
			}

			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return Foo{
					Name: "test",
				}, nil
			})

			var output struct {
				Attr1 Foo `env:"attr1"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Attr1, Foo{
				Name: "test",
			})
		})

		t.Run("should work with embeded fields", func(t *testing.T) {
			type Foo struct {
				Name      string
				IsEmbeded bool
			}

			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return Foo{
					Name:      field.Name,      // should be foo
					IsEmbeded: field.IsEmbeded, // should be true
				}, nil
			})

			var output struct {
				Foo `env:"attr1"`
			}
			err := ss.Decode(&output, decoder)
			tt.AssertNoErr(t, err)
			tt.AssertEqual(t, output.Foo, Foo{
				Name:      "Foo",
				IsEmbeded: true,
			})
		})
	})

	t.Run("should report errors correctly", func(t *testing.T) {
		tests := []struct {
			desc               string
			value              interface{}
			targetStruct       interface{}
			expectErrToContain []string
		}{
			{
				desc:               "should report error input is a ptr to something else than a struct",
				value:              "example-value",
				targetStruct:       &[]int{},
				expectErrToContain: []string{"can only get struct info from structs", "[]int"},
			},
			{
				desc:  "should report error if input is not a pointer",
				value: "example-value",
				targetStruct: struct {
					Attr1 string `some_tag:""`
				}{},
				expectErrToContain: []string{"expected struct pointer"},
			},
			{
				desc:  "should report error if input a nil ptr to struct",
				value: "example-value",
				targetStruct: (*struct {
					Attr1 string `some_tag:""`
				})(nil),
				expectErrToContain: []string{"expected non-nil pointer"},
			},
			{
				desc:  "should report error if the type doesnt match",
				value: "example-value",
				targetStruct: &struct {
					Attr1 int `env:"attr1"`
				}{},
				expectErrToContain: []string{"string", "int"},
			},
			{
				desc:  "should report error if parsing a non-slice into a slice field",
				value: "example-value",
				targetStruct: &struct {
					Attr1 []string `some_tag:"attr1"`
				}{},
				expectErrToContain: []string{"expected slice", "Attr1", "string", "example-value"},
			},
			{
				desc:  "should report error if the conversion fails for one of the slice elements",
				value: []any{42, "not a number", 43},
				targetStruct: &struct {
					Attr1 []int `some_tag:"attr1"`
				}{},
				expectErrToContain: []string{"error converting", "Attr1", "int", "string"},
			},
			{
				desc:  "should report error if tag has no name",
				value: "example-value",
				targetStruct: &struct {
					Attr1 string `valid:"attr1" :"missing_name"`
				}{},
				expectErrToContain: []string{"malformed tag", `valid:"attr1" :"missing_name"`},
			},
			{
				desc:  "should report error if tag has no value",
				value: "example-value",
				targetStruct: &struct {
					Attr1 string `valid:"attr1" missing_value:`
				}{},
				expectErrToContain: []string{"malformed tag", `valid:"attr1" missing_value:`},
			},
			{
				desc:  "should report error if tag has invalid character",
				value: "example-value",
				targetStruct: &struct {
					Attr1 string `line_break
												"attr1"`
				}{},
				// (10 is the ascii number for line breaks)
				expectErrToContain: []string{"malformed tag", "10"},
			},
			{
				desc:  "should report error if tag value is missing quotes",
				value: "example-value",
				targetStruct: &struct {
					Attr1 string `line_break:attr1"`
				}{},
				expectErrToContain: []string{"malformed tag", "missing quotes", `line_break:attr1"`},
			},
			{
				desc:  "should report error if tag value is missing quotes",
				value: "example-value",
				targetStruct: &struct {
					Attr1 string `line_break:"attr1`
				}{},
				expectErrToContain: []string{"malformed tag", "missing end quote", `line_break:"attr1`},
			},
		}
		for _, test := range tests {
			t.Run(test.desc, func(t *testing.T) {
				decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
					return test.value, nil
				})

				err := ss.Decode(test.targetStruct, decoder)
				tt.AssertErrContains(t, err, test.expectErrToContain...)
			})
		}
	})

	t.Run("wrap errors correctly", func(t *testing.T) {

		t.Run("wrap error from Decoder", func(t *testing.T) {
			// Use a int parse error as the type example
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return strconv.ParseInt("not-an-int", 10, 0)
			})
			err := ss.Decode(&struct {
				A int `a:""`
			}{}, decoder)

			var parseErr *strconv.NumError
			tt.AssertTrue(t, errors.As(err, &parseErr), "error %#v should wrap %T", err, parseErr)
			tt.AssertEqual(t, parseErr.Err, strconv.ErrSyntax)
		})

		t.Run("wrap error from nested Decoder", func(t *testing.T) {
			// Use a int parse error as the type example
			var decoder ss.TagDecoder
			decoder = ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				if field.Name == "A" {
					// Recurse
					return decoder, nil
				}
				return strconv.ParseInt("not-an-int", 10, 0)
			})

			type Outer struct {
				A struct{ B int }
			}
			err := ss.Decode(&Outer{}, decoder)

			// Sanity check: the outer error _does_ contain the string we don't want to see in the wrapped error
			tt.AssertErrContains(t, err, "error decoding nested field")

			var parseErr *strconv.NumError
			tt.AssertTrue(t, errors.As(err, &parseErr), "error %#v should wrap %T", err, parseErr)
			tt.AssertEqual(t, parseErr.Err, strconv.ErrSyntax)
		})

		t.Run("wrap error from slice conversion", func(t *testing.T) {
			// Use a int parse error as the type example
			decoder := ss.FuncTagDecoder(func(field ss.Field) (interface{}, error) {
				return []string{"not-an-int"}, nil
			})
			err := ss.Decode(&struct {
				A []int `a:""`
			}{}, decoder)

			// Sanity check: the outer error _does_ contain the string we don't want to see in the wrapped error
			tt.AssertErrContains(t, err, "error converting A[0]")

			// In this case, it should just be a wrapped sting error
			wrapped := errors.Unwrap(err)
			tt.AssertNotEqual(t, wrapped, nil)
		})
	})
}

func TestGetStructInfo(t *testing.T) {
	type MyStruct struct {
		A int
	}

	t.Run("should work for a struct pointer", func(t *testing.T) {
		var s MyStruct
		si, err := ss.GetStructInfo(&s)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, len(si.Fields), 1)
	})

	t.Run("should fail for a struct", func(t *testing.T) {
		var s MyStruct
		_, err := ss.GetStructInfo(s)
		tt.AssertErrContains(t, err, "struct pointer", "MyStruct")
	})

	t.Run("should work for reflect.Type", func(t *testing.T) {
		typ := reflect.TypeOf(MyStruct{})
		si, err := ss.GetStructInfo(typ)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, len(si.Fields), 1)
	})
	t.Run("should work for reflect.Type of a struct pointer", func(t *testing.T) {
		typ := reflect.TypeOf(&MyStruct{})
		si, err := ss.GetStructInfo(typ)
		tt.AssertNoErr(t, err)
		tt.AssertEqual(t, len(si.Fields), 1)
	})
	t.Run("should fa for reflect.Type of not a struct", func(t *testing.T) {
		typ := reflect.TypeOf(1)
		_, err := ss.GetStructInfo(typ)
		tt.AssertErrContains(t, err, "can only get struct info from structs", `"int"`)
	})
}

func intPtr(i int) *int {
	return &i
}
