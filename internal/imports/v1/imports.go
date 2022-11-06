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

func HostFunctions(instance common.WasmInstance) map[string]interface{} {
	hostFuncs := map[string]interface{}{}
	h := &host{Instance: instance}

	hostFuncs["proxy_log"] = h.ProxyLog
	hostFuncs["proxy_set_effective_context"] = h.ProxySetEffectiveContext

	hostFuncs["proxy_get_property"] = h.ProxyGetProperty
	hostFuncs["proxy_set_property"] = h.ProxySetProperty

	hostFuncs["proxy_get_buffer_bytes"] = h.ProxyGetBufferBytes
	hostFuncs["proxy_set_buffer_bytes"] = h.ProxySetBufferBytes

	hostFuncs["proxy_get_header_map_pairs"] = h.ProxyGetHeaderMapPairs
	hostFuncs["proxy_set_header_map_pairs"] = h.ProxySetHeaderMapPairs

	hostFuncs["proxy_get_header_map_value"] = h.ProxyGetHeaderMapValue
	hostFuncs["proxy_replace_header_map_value"] = h.ProxyReplaceHeaderMapValue
	hostFuncs["proxy_add_header_map_value"] = h.ProxyAddHeaderMapValue
	hostFuncs["proxy_remove_header_map_value"] = h.ProxyRemoveHeaderMapValue

	hostFuncs["proxy_set_tick_period_milliseconds"] = h.ProxySetTickPeriodMilliseconds
	hostFuncs["proxy_get_current_time_nanoseconds"] = h.ProxyGetCurrentTimeNanoseconds

	hostFuncs["proxy_resume_downstream"] = h.ProxyResumeDownstream
	hostFuncs["proxy_resume_upstream"] = h.ProxyResumeUpstream

	hostFuncs["proxy_open_grpc_stream"] = h.ProxyOpenGrpcStream
	hostFuncs["proxy_send_grpc_call_message"] = h.ProxySendGrpcCallMessage
	hostFuncs["proxy_cancel_grpc_call"] = h.ProxyCancelGrpcCall
	hostFuncs["proxy_close_grpc_call"] = h.ProxyCloseGrpcCall

	hostFuncs["proxy_grpc_call"] = h.ProxyGrpcCall
	hostFuncs["proxy_dispatch_grpc_call"] = h.ProxyGrpcCall

	hostFuncs["proxy_resume_http_request"] = h.ProxyResumeHttpRequest
	hostFuncs["proxy_resume_http_response"] = h.ProxyResumeHttpResponse
	hostFuncs["proxy_send_http_response"] = h.ProxySendHttpResponse

	hostFuncs["proxy_http_call"] = h.ProxyHttpCall
	hostFuncs["proxy_dispatch_http_call"] = h.ProxyHttpCall

	hostFuncs["proxy_define_metric"] = h.ProxyDefineMetric
	hostFuncs["proxy_remove_metric"] = h.ProxyRemoveMetric
	hostFuncs["proxy_increment_metric"] = h.ProxyIncrementMetric
	hostFuncs["proxy_record_metric"] = h.ProxyRecordMetric
	hostFuncs["proxy_get_metric"] = h.ProxyGetMetric

	hostFuncs["proxy_register_shared_queue"] = h.ProxyRegisterSharedQueue
	hostFuncs["proxy_remove_shared_queue"] = h.ProxyRemoveSharedQueue
	hostFuncs["proxy_resolve_shared_queue"] = h.ProxyResolveSharedQueue
	hostFuncs["proxy_dequeue_shared_queue"] = h.ProxyDequeueSharedQueue
	hostFuncs["proxy_enqueue_shared_queue"] = h.ProxyEnqueueSharedQueue

	hostFuncs["proxy_get_shared_data"] = h.ProxyGetSharedData
	hostFuncs["proxy_set_shared_data"] = h.ProxySetSharedData

	hostFuncs["proxy_done"] = h.ProxyDone

	hostFuncs["proxy_call_foreign_function"] = h.ProxyCallForeignFunction

	return hostFuncs
}

type host struct {
	Instance common.WasmInstance
}

func (h *host) ProxyLog(ctx context.Context, level int32, logDataPtr int32, logDataSize int32) int32 {
	instance := h.Instance
	logContent, err := instance.GetMemory(uint64(logDataPtr), uint64(logDataSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	callback := getImportHandler(instance)

	return callback.Log(v1.LogLevel(level), string(logContent)).Int32()
}

func (h *host) ProxySetEffectiveContext(ctx context.Context, contextID int32) int32 {
	ih := getImportHandler(h.Instance)

	return ih.SetEffectiveContextID(contextID).Int32()
}

func (h *host) ProxySetTickPeriodMilliseconds(ctx context.Context, tickPeriodMilliseconds int32) int32 {
	ih := getImportHandler(h.Instance)

	return ih.SetTickPeriodMilliseconds(tickPeriodMilliseconds).Int32()
}

func (h *host) ProxyGetCurrentTimeNanoseconds(ctx context.Context, resultUint64Ptr int32) int32 {
	instance := h.Instance
	ih := getImportHandler(instance)

	nano, res := ih.GetCurrentTimeNanoseconds()
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err := instance.PutUint32(uint64(resultUint64Ptr), uint32(nano))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxyDone(ctx context.Context) int32 {
	ih := getImportHandler(h.Instance)

	return ih.Done().Int32()
}

func (h *host) ProxyCallForeignFunction(ctx context.Context, funcNamePtr int32, funcNameSize int32,
	paramPtr int32, paramSize int32, returnData int32, returnSize int32,
) int32 {
	instance := h.Instance
	funcName, err := instance.GetMemory(uint64(funcNamePtr), uint64(funcNameSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	param, err := instance.GetMemory(uint64(paramPtr), uint64(paramSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	ret, res := ih.CallForeignFunction(string(funcName), param)
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	return copyBytesIntoInstance(instance, ret, returnData, returnSize).Int32()
}
