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

func (h *host) ProxyResumeHttpRequest(ctx context.Context) int32 {
	ih := getImportHandler(h.Instance)

	return ih.ResumeHttpRequest().Int32()
}

func (h *host) ProxyResumeHttpResponse(ctx context.Context) int32 {
	ih := getImportHandler(h.Instance)

	return ih.ResumeHttpResponse().Int32()
}

func (h *host) ProxySendHttpResponse(ctx context.Context, respCode int32, respCodeDetailPtr int32, respCodeDetailSize int32,
	respBodyPtr int32, respBodySize int32, additionalHeaderMapDataPtr int32, additionalHeaderSize int32, grpcStatus int32,
) int32 {
	instance := h.Instance
	respCodeDetail, err := instance.GetMemory(uint64(respCodeDetailPtr), uint64(respCodeDetailSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	respBody, err := instance.GetMemory(uint64(respBodyPtr), uint64(respBodySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	additionalHeaderMapData, err := instance.GetMemory(uint64(additionalHeaderMapDataPtr), uint64(additionalHeaderSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	additionalHeaderMap := common.DecodeMap(additionalHeaderMapData)

	ih := getImportHandler(instance)

	return ih.SendHttpResp(respCode,
		common.NewIoBufferBytes(respCodeDetail),
		common.NewIoBufferBytes(respBody),
		common.CommonHeader(additionalHeaderMap), grpcStatus).Int32()
}

func (h *host) ProxyHttpCall(ctx context.Context, uriPtr int32, uriSize int32,
	headerPairsPtr int32, headerPairsSize int32, bodyPtr int32, bodySize int32, trailerPairsPtr int32, trailerPairsSize int32,
	timeoutMilliseconds int32, calloutIDPtr int32,
) int32 {
	instance := h.Instance
	url, err := instance.GetMemory(uint64(uriPtr), uint64(uriSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	headerMapData, err := instance.GetMemory(uint64(headerPairsPtr), uint64(headerPairsSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	headerMap := common.DecodeMap(headerMapData)

	body, err := instance.GetMemory(uint64(bodyPtr), uint64(bodySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	trailerMapData, err := instance.GetMemory(uint64(trailerPairsPtr), uint64(trailerPairsSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	trailerMap := common.DecodeMap(trailerMapData)

	ih := getImportHandler(instance)

	calloutID, res := ih.HttpCall(
		string(url),
		common.CommonHeader(headerMap),
		common.NewIoBufferBytes(body),
		common.CommonHeader(trailerMap),
		timeoutMilliseconds,
	)
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(calloutIDPtr), uint32(calloutID))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}
