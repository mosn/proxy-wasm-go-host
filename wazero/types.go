/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package wazero

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

func convertFromGoValue(a interface{}) (t api.ValueType, v uint64, err error) {
	switch a := a.(type) {
	case int32:
		v = api.EncodeI32(a)
		t = api.ValueTypeI32
	case int64:
		v = api.EncodeI64(a)
		t = api.ValueTypeI64
	case float32:
		v = api.EncodeF32(a)
		t = api.ValueTypeF32
	case float64:
		v = api.EncodeF64(a)
		t = api.ValueTypeF64
	default:
		err = fmt.Errorf("unexpected arg type %v", a)
	}
	return
}

func convertToGoValue(t api.ValueType, v uint64) (interface{}, error) {
	switch t {
	case api.ValueTypeI32:
		return int32(v), nil
	case api.ValueTypeI64:
		return int64(v), nil
	case api.ValueTypeF32:
		return api.DecodeF32(v), nil
	case api.ValueTypeF64:
		return api.DecodeF64(v), nil
	default:
		return nil, fmt.Errorf("[wazero][type] convertToGoType unsupported type: %v", api.ValueTypeName(t))
	}
}
