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

func (h *host) ProxyOpenGrpcStream(ctx context.Context, grpcServiceData int32, grpcServiceSize int32,
	serviceNameData int32, serviceNameSize int32, methodData int32, methodSize int32, returnCalloutID int32,
) int32 {
	instance := h.Instance
	grpcService, err := instance.GetMemory(uint64(grpcServiceData), uint64(grpcServiceSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	serviceName, err := instance.GetMemory(uint64(serviceNameData), uint64(serviceNameSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	method, err := instance.GetMemory(uint64(methodData), uint64(methodSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	calloutID, res := ih.OpenGrpcStream(string(grpcService), string(serviceName), string(method))
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(returnCalloutID), uint32(calloutID))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxySendGrpcCallMessage(ctx context.Context, calloutID int32, data int32, size int32, endOfStream int32) int32 {
	instance := h.Instance
	msg, err := instance.GetMemory(uint64(data), uint64(size))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	return ih.SendGrpcCallMsg(calloutID, common.NewIoBufferBytes(msg), endOfStream).Int32()
}

func (h *host) ProxyCancelGrpcCall(ctx context.Context, calloutID int32) int32 {
	instance := h.Instance
	ih := getImportHandler(instance)

	return ih.CancelGrpcCall(calloutID).Int32()
}

func (h *host) ProxyCloseGrpcCall(ctx context.Context, calloutID int32) int32 {
	instance := h.Instance
	ih := getImportHandler(instance)

	return ih.CloseGrpcCall(calloutID).Int32()
}

func (h *host) ProxyGrpcCall(ctx context.Context, grpcServiceData int32, grpcServiceSize int32,
	serviceNameData int32, serviceNameSize int32,
	methodData int32, methodSize int32,
	grpcMessageData int32, grpcMessageSize int32,
	timeoutMilliseconds int32, returnCalloutID int32,
) int32 {
	instance := h.Instance
	grpcService, err := instance.GetMemory(uint64(grpcServiceData), uint64(grpcServiceSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	serviceName, err := instance.GetMemory(uint64(serviceNameData), uint64(serviceNameSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	method, err := instance.GetMemory(uint64(methodData), uint64(methodSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	msg, err := instance.GetMemory(uint64(grpcMessageData), uint64(grpcMessageSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	calloutID, res := ih.GrpcCall(string(grpcService), string(serviceName), string(method),
		common.NewIoBufferBytes(msg), timeoutMilliseconds)
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(returnCalloutID), uint32(calloutID))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}
