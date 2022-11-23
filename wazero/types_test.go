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
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero/api"
)

func TestTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected api.ValueType
	}{
		{name: "int32", input: int32(math.MinInt32), expected: api.ValueTypeI32},
		{name: "int64", input: int64(math.MinInt64), expected: api.ValueTypeI64},
		{name: "float32", input: float32(math.MaxFloat32), expected: api.ValueTypeF32},
		{name: "float64", input: math.MaxFloat64, expected: api.ValueTypeF64},
	}

	for _, tt := range tests {
		tc := tt

		t.Run(tc.name, func(t *testing.T) {
			vt, v, err := convertFromGoValue(tc.input)
			require.NoError(t, err)
			require.Equal(t, vt, tc.expected)

			g, err := convertToGoValue(vt, v)
			require.NoError(t, err)
			require.Equal(t, g, tc.input)
		})
	}
}
