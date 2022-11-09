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

package v1

import (
	"context"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
)

// u32 is the fixed size of a uint32 in little-endian encoding.
const u32Len = 4

func GetMap(instance common.WasmInstance, mapType v1.MapType) common.HeaderMap {
	ih := getImportHandler(instance)

	switch mapType {
	case v1.MapTypeHttpRequestHeaders:
		return ih.GetHttpRequestHeader()
	case v1.MapTypeHttpRequestTrailers:
		return ih.GetHttpRequestTrailer()
	case v1.MapTypeHttpResponseHeaders:
		return ih.GetHttpResponseHeader()
	case v1.MapTypeHttpResponseTrailers:
		return ih.GetHttpResponseTrailer()
	case v1.MapTypeGrpcReceiveInitialMetadata:
		return ih.GetGrpcReceiveInitialMetaData()
	case v1.MapTypeGrpcReceiveTrailingMetadata:
		return ih.GetGrpcReceiveTrailerMetaData()
	case v1.MapTypeHttpCallResponseHeaders:
		return ih.GetHttpCallResponseHeaders()
	case v1.MapTypeHttpCallResponseTrailers:
		return ih.GetHttpCallResponseTrailer()
	default:
		return ih.GetCustomHeader(mapType)
	}
}

func (h *host) ProxyGetHeaderMapPairs(ctx context.Context, mapType int32, returnDataPtr int32, returnDataSize int32) int32 {
	instance := h.Instance
	header := GetMap(instance, v1.MapType(mapType))
	if header == nil {
		return v1.WasmResultNotFound.Int32()
	}

	cloneMap := make(map[string]string)
	totalBytesLen := u32Len
	header.Range(func(key, value string) bool {
		cloneMap[key] = value
		totalBytesLen += u32Len + u32Len               // keyLen + valueLen
		totalBytesLen += len(key) + 1 + len(value) + 1 // key + \0 + value + \0

		return true
	})

	addr, err := instance.Malloc(int32(totalBytesLen))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(addr, uint32(len(cloneMap)))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	lenPtr := addr + u32Len
	dataPtr := lenPtr + uint64((u32Len+u32Len)*len(cloneMap))

	for k, v := range cloneMap {
		_ = instance.PutUint32(lenPtr, uint32(len(k)))
		lenPtr += u32Len
		_ = instance.PutUint32(lenPtr, uint32(len(v)))
		lenPtr += u32Len

		_ = instance.PutMemory(dataPtr, uint64(len(k)), []byte(k))
		dataPtr += uint64(len(k))
		_ = instance.PutByte(dataPtr, 0)
		dataPtr++

		_ = instance.PutMemory(dataPtr, uint64(len(v)), []byte(v))
		dataPtr += uint64(len(v))
		_ = instance.PutByte(dataPtr, 0)
		dataPtr++
	}

	err = instance.PutUint32(uint64(returnDataPtr), uint32(addr))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(returnDataSize), uint32(totalBytesLen))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxySetHeaderMapPairs(ctx context.Context, mapType int32, ptr int32, size int32) int32 {
	instance := h.Instance
	headerMap := GetMap(instance, v1.MapType(mapType))
	if headerMap == nil {
		return v1.WasmResultNotFound.Int32()
	}

	newMapContent, err := instance.GetMemory(uint64(ptr), uint64(size))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	newMap := common.DecodeMap(newMapContent)

	for k, v := range newMap {
		headerMap.Set(k, v)
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxyGetHeaderMapValue(ctx context.Context, mapType int32, keyDataPtr int32, keySize int32, valueDataPtr int32, valueSize int32) int32 {
	instance := h.Instance
	headerMap := GetMap(instance, v1.MapType(mapType))
	if headerMap == nil {
		return v1.WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	value, ok := headerMap.Get(string(key))
	if !ok {
		return v1.WasmResultNotFound.Int32()
	}

	return copyIntoInstance(instance, value, valueDataPtr, valueSize).Int32()
}

func (h *host) ProxyReplaceHeaderMapValue(ctx context.Context, mapType int32, keyDataPtr int32, keySize int32, valueDataPtr int32, valueSize int32) int32 {
	instance := h.Instance
	headerMap := GetMap(instance, v1.MapType(mapType))
	if headerMap == nil {
		return v1.WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valueDataPtr), uint64(valueSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(value) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	headerMap.Set(string(key), string(value))

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxyAddHeaderMapValue(ctx context.Context, mapType int32, keyDataPtr int32, keySize int32, valueDataPtr int32, valueSize int32) int32 {
	instance := h.Instance
	headerMap := GetMap(instance, v1.MapType(mapType))
	if headerMap == nil {
		return v1.WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valueDataPtr), uint64(valueSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	headerMap.Set(string(key), string(value))

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxyRemoveHeaderMapValue(ctx context.Context, mapType int32, keyDataPtr int32, keySize int32) int32 {
	instance := h.Instance
	headerMap := GetMap(instance, v1.MapType(mapType))
	if headerMap == nil {
		return v1.WasmResultNotFound.Int32()
	}

	key, err := instance.GetMemory(uint64(keyDataPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	headerMap.Del(string(key))

	return v1.WasmResultOk.Int32()
}
