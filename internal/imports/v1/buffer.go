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

func GetBuffer(instance common.WasmInstance, bufferType v1.BufferType) common.IoBuffer {
	im := getImportHandler(instance)

	switch bufferType {
	case v1.BufferTypeHttpRequestBody:
		return im.GetHttpRequestBody()
	case v1.BufferTypeHttpResponseBody:
		return im.GetHttpResponseBody()
	case v1.BufferTypeDownstreamData:
		return im.GetDownStreamData()
	case v1.BufferTypeUpstreamData:
		return im.GetUpstreamData()
	case v1.BufferTypeHttpCallResponseBody:
		return im.GetHttpCallResponseBody()
	case v1.BufferTypeGrpcReceiveBuffer:
		return im.GetGrpcReceiveBuffer()
	case v1.BufferTypePluginConfiguration:
		return im.GetPluginConfig()
	case v1.BufferTypeVmConfiguration:
		return im.GetVmConfig()
	case v1.BufferTypeCallData:
		return im.GetFuncCallData()
	default:
		return im.GetCustomBuffer(bufferType)
	}
}

func (h *host) ProxyGetBufferBytes(ctx context.Context, bufferType int32, start int32, length int32, returnBufferData int32, returnBufferSize int32) int32 {
	instance := h.Instance
	buf := GetBuffer(instance, v1.BufferType(bufferType))
	if buf == nil {
		return v1.WasmResultNotFound.Int32()
	}

	if start > start+length {
		return v1.WasmResultBadArgument.Int32()
	}

	if start+length > int32(buf.Len()) {
		length = int32(buf.Len()) - start
	}

	addr, err := instance.Malloc(length)
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutMemory(addr, uint64(length), buf.Bytes())
	if err != nil {
		return v1.WasmResultInternalFailure.Int32()
	}

	err = instance.PutUint32(uint64(returnBufferData), uint32(addr))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	err = instance.PutUint32(uint64(returnBufferSize), uint32(length))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxySetBufferBytes(ctx context.Context, bufferType int32, start int32, length int32, dataPtr int32, dataSize int32) int32 {
	instance := h.Instance
	buf := GetBuffer(instance, v1.BufferType(bufferType))
	if buf == nil {
		return v1.WasmResultNotFound.Int32()
	}

	content, err := instance.GetMemory(uint64(dataPtr), uint64(dataSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	switch {
	case start == 0:
		if length == 0 || int(length) >= buf.Len() {
			buf.Drain(buf.Len())
			_, err = buf.Write(content)
		} else {
			return v1.WasmResultBadArgument.Int32()
		}
	case int(start) >= buf.Len():
		_, err = buf.Write(content)
	default:
		return v1.WasmResultBadArgument.Int32()
	}

	if err != nil {
		return v1.WasmResultInternalFailure.Int32()
	}

	return v1.WasmResultOk.Int32()
}
