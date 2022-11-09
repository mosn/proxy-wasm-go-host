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

package common

import (
	"encoding/binary"
)

// u32 is the fixed size of a uint32 in little-endian encoding.
const u32Len = 4

// EncodeMap encode map into bytes.
func EncodeMap(m map[string]string) []byte {
	if len(m) == 0 {
		return nil
	}

	totalBytesLen := u32Len
	for k, v := range m {
		totalBytesLen += u32Len + u32Len
		totalBytesLen += len(k) + 1 + len(v) + 1
	}

	b := make([]byte, totalBytesLen)
	binary.LittleEndian.PutUint32(b, uint32(len(m)))

	lenPtr := u32Len
	dataPtr := lenPtr + (u32Len+u32Len)*len(m)

	for k, v := range m {
		binary.LittleEndian.PutUint32(b[lenPtr:], uint32(len(k)))
		lenPtr += u32Len
		binary.LittleEndian.PutUint32(b[lenPtr:], uint32(len(v)))
		lenPtr += u32Len

		copy(b[dataPtr:], k)
		dataPtr += len(k)
		b[dataPtr] = '0'
		dataPtr++

		copy(b[dataPtr:], v)
		dataPtr += len(v)
		b[dataPtr] = '0'
		dataPtr++
	}

	return b
}

// DecodeMap decode map from rawData.
func DecodeMap(rawData []byte) map[string]string {
	if len(rawData) < u32Len {
		return nil
	}

	headerSize := binary.LittleEndian.Uint32(rawData[0:u32Len])

	// headerSize + (key1_size + value1_size) * headerSize
	dataPtr := u32Len + (u32Len+u32Len)*int(headerSize)
	if dataPtr >= len(rawData) {
		return nil
	}

	res := make(map[string]string, headerSize)

	for i := 0; i < int(headerSize); i++ {
		lenIndex := u32Len + (u32Len+u32Len)*i
		keySize := int(binary.LittleEndian.Uint32(rawData[lenIndex : lenIndex+u32Len]))
		valueSize := int(binary.LittleEndian.Uint32(rawData[lenIndex+u32Len : lenIndex+u32Len+u32Len]))

		if dataPtr >= len(rawData) || dataPtr+keySize > len(rawData) {
			break
		}

		key := string(rawData[dataPtr : dataPtr+keySize])
		dataPtr += keySize
		dataPtr++ // 0

		if dataPtr >= len(rawData) || dataPtr+keySize > len(rawData) {
			break
		}

		value := string(rawData[dataPtr : dataPtr+valueSize])
		dataPtr += valueSize
		dataPtr++ // 0

		res[key] = value
	}

	return res
}
