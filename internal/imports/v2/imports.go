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

package v2

import (
	"context"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v2 "mosn.io/proxy-wasm-go-host/proxywasm/v2"
)

// u32 is the fixed size of a uint32 in little-endian encoding.
const u32Len = 4

func HostFunctions(instance common.WasmInstance) map[string]interface{} {
	hostFuncs := map[string]interface{}{}
	h := &host{Instance: instance}

	hostFuncs["proxy_log"] = h.ProxyLog

	hostFuncs["proxy_set_effective_context"] = h.ProxySetEffectiveContext
	hostFuncs["proxy_context_finalize"] = h.ProxyContextFinalize

	hostFuncs["proxy_resume_stream"] = h.ProxyResumeStream
	hostFuncs["proxy_close_stream"] = h.ProxyCloseStream

	hostFuncs["proxy_send_http_response"] = h.ProxySendHttpResponse
	hostFuncs["proxy_resume_http_stream"] = h.ProxyResumeHttpStream
	hostFuncs["proxy_close_http_stream"] = h.ProxyCloseHttpStream

	hostFuncs["proxy_get_buffer_bytes"] = h.ProxyGetBuffer

	hostFuncs["proxy_get_buffer"] = h.ProxyGetBuffer
	hostFuncs["proxy_set_buffer"] = h.ProxySetBuffer

	hostFuncs["proxy_get_header_map_pairs"] = h.ProxyGetHeaderMapPairs

	hostFuncs["proxy_get_header_map_value"] = h.ProxyGetHeaderMapValue
	hostFuncs["proxy_replace_header_map_value"] = h.ProxyReplaceHeaderMapValue

	hostFuncs["proxy_open_shared_kvstore"] = h.ProxyOpenSharedKvstore
	hostFuncs["proxy_get_shared_kvstore_key_values"] = h.ProxyGetSharedKvstoreKeyValues
	hostFuncs["proxy_set_shared_kvstore_key_values"] = h.ProxySetSharedKvstoreKeyValues
	hostFuncs["proxy_add_shared_kvstore_key_values"] = h.ProxyAddSharedKvstoreKeyValues
	hostFuncs["proxy_remove_shared_kvstore_key"] = h.ProxyRemoveSharedKvstoreKey
	hostFuncs["proxy_delete_shared_kvstore"] = h.ProxyDeleteSharedKvstore

	hostFuncs["proxy_open_shared_queue"] = h.ProxyOpenSharedQueue
	hostFuncs["proxy_dequeue_shared_queue_item"] = h.ProxyDequeueSharedQueueItem
	hostFuncs["proxy_enqueue_shared_queue_item"] = h.ProxyEnqueueSharedQueueItem
	hostFuncs["proxy_delete_shared_queue"] = h.ProxyDeleteSharedQueue

	hostFuncs["proxy_create_timer"] = h.ProxyCreateTimer
	hostFuncs["proxy_delete_timer"] = h.ProxyDeleteTimer

	hostFuncs["proxy_create_metric"] = h.ProxyCreateMetric
	hostFuncs["proxy_get_metric_value"] = h.ProxyGetMetricValue
	hostFuncs["proxy_set_metric_value"] = h.ProxySetMetricValue
	hostFuncs["proxy_increment_metric_value"] = h.ProxyIncrementMetricValue
	hostFuncs["proxy_delete_metric"] = h.ProxyDeleteMetric

	hostFuncs["proxy_http_call"] = h.ProxyDispatchHttpCall
	hostFuncs["proxy_dispatch_http_call"] = h.ProxyDispatchHttpCall

	hostFuncs["proxy_dispatch_grpc_call"] = h.ProxyDispatchGrpcCall
	hostFuncs["proxy_open_grpc_stream"] = h.ProxyOpenGrpcStream
	hostFuncs["proxy_send_grpc_stream_message"] = h.ProxySendGrpcStreamMessage
	hostFuncs["proxy_cancel_grpc_call"] = h.ProxyCancelGrpcCall
	hostFuncs["proxy_close_grpc_call"] = h.ProxyCloseGrpcCall

	hostFuncs["proxy_call_custom_function"] = h.ProxyCallCustomFunction

	return hostFuncs
}

type host struct {
	Instance common.WasmInstance
}

func (h *host) ProxyLog(ctx context.Context, logLevel int32, messageData int32, messageSize int32) v2.Result {
	instance := h.Instance
	msg, err := instance.GetMemory(uint64(messageData), uint64(messageSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	callback := getImportHandler(instance)

	return callback.Log(v2.LogLevel(logLevel), string(msg))
}

func (h *host) ProxySetEffectiveContext(ctx context.Context, contextID int32) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.SetEffectiveContext(contextID)
}

func (h *host) ProxyContextFinalize(ctx context.Context) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.ContextFinalize()
}

func (h *host) ProxyResumeStream(ctx context.Context, streamType v2.StreamType) v2.Result {
	callback := getImportHandler(h.Instance)
	switch streamType {
	case v2.StreamTypeDownstream:
		return callback.ResumeDownStream()
	case v2.StreamTypeUpstream:
		return callback.ResumeUpStream()
	case v2.StreamTypeHttpRequest:
		return callback.ResumeHttpRequest()
	case v2.StreamTypeHttpResponse:
		return callback.ResumeHttpResponse()
	default:
		return callback.ResumeCustomStream(streamType)
	}
}

func (h *host) ProxyCloseStream(ctx context.Context, streamType v2.StreamType) v2.Result {
	callback := getImportHandler(h.Instance)
	switch streamType {
	case v2.StreamTypeDownstream:
		return callback.CloseDownStream()
	case v2.StreamTypeUpstream:
		return callback.CloseUpStream()
	case v2.StreamTypeHttpRequest:
		return callback.CloseHttpRequest()
	case v2.StreamTypeHttpResponse:
		return callback.CloseHttpResponse()
	default:
		return callback.CloseCustomStream(streamType)
	}
}

func (h *host) ProxySendHttpResponse(ctx context.Context, responseCode int32, responseCodeDetailsData int32, responseCodeDetailsSize int32,
	responseBodyData int32, responseBodySize int32, additionalHeadersMapData int32, additionalHeadersSize int32,
	grpcStatus int32,
) v2.Result {
	instance := h.Instance
	respCodeDetail, err := instance.GetMemory(uint64(responseCodeDetailsData), uint64(responseCodeDetailsSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	respBody, err := instance.GetMemory(uint64(responseBodyData), uint64(responseBodySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	additionalHeaderMapData, err := instance.GetMemory(uint64(additionalHeadersMapData), uint64(additionalHeadersSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	additionalHeaderMap := common.DecodeMap(additionalHeaderMapData)

	callback := getImportHandler(instance)

	return callback.SendHttpResp(responseCode,
		common.NewIoBufferBytes(respCodeDetail),
		common.NewIoBufferBytes(respBody),
		common.CommonHeader(additionalHeaderMap), grpcStatus)
}

func (h *host) ProxyResumeHttpStream(ctx context.Context, streamType v2.StreamType) v2.Result {
	callback := getImportHandler(h.Instance)
	switch streamType {
	case v2.StreamTypeHttpRequest:
		return callback.ResumeHttpRequest()
	case v2.StreamTypeHttpResponse:
		return callback.ResumeHttpResponse()
	}

	return v2.ResultBadArgument
}

func (h *host) ProxyCloseHttpStream(ctx context.Context, streamType v2.StreamType) v2.Result {
	callback := getImportHandler(h.Instance)
	switch streamType {
	case v2.StreamTypeHttpRequest:
		return callback.CloseHttpRequest()
	case v2.StreamTypeHttpResponse:
		return callback.CloseHttpResponse()
	}

	return v2.ResultBadArgument
}

func GetBuffer(instance common.WasmInstance, bufferType v2.BufferType) common.IoBuffer {
	im := getImportHandler(instance)

	switch bufferType {
	case v2.BufferTypeHttpRequestBody:
		return im.GetHttpRequestBody()
	case v2.BufferTypeHttpResponseBody:
		return im.GetHttpResponseBody()
	case v2.BufferTypeDownstreamData:
		return im.GetDownStreamData()
	case v2.BufferTypeUpstreamData:
		return im.GetUpstreamData()
	case v2.BufferTypeHttpCalloutResponseBody:
		return im.GetHttpCalloutResponseBody()
	case v2.BufferTypePluginConfiguration:
		return im.GetPluginConfig()
	case v2.BufferTypeVmConfiguration:
		return im.GetVmConfig()
	case v2.BufferTypeHttpCallResponseBody:
		return im.GetHttpCalloutResponseBody()
	default:
		return im.GetCustomBuffer(bufferType)
	}
}

func (h *host) ProxyGetBuffer(ctx context.Context, bufferType int32, offset int32, maxSize int32,
	returnBufferData int32, returnBufferSize int32,
) v2.Result {
	instance := h.Instance
	buf := GetBuffer(instance, v2.BufferType(bufferType))
	if buf == nil {
		return v2.ResultBadArgument
	}

	if buf.Len() == 0 {
		return v2.ResultEmpty
	}

	if offset > offset+maxSize {
		return v2.ResultBadArgument
	}

	if offset+maxSize > int32(buf.Len()) {
		maxSize = int32(buf.Len()) - offset
	}

	return copyIntoInstance(instance, buf.Bytes()[offset:offset+maxSize], returnBufferData, returnBufferSize)
}

func (h *host) ProxySetBuffer(ctx context.Context, bufferType v2.BufferType, offset int32, size int32,
	bufferData int32, bufferSize int32,
) v2.Result {
	instance := h.Instance
	buf := GetBuffer(instance, bufferType)
	if buf == nil {
		return v2.ResultBadArgument
	}

	content, err := instance.GetMemory(uint64(bufferData), uint64(bufferSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	switch {
	case offset == 0:
		if size == 0 || int(size) >= buf.Len() {
			buf.Drain(buf.Len())
			_, err = buf.Write(content)
		} else {
			return v2.ResultBadArgument
		}
	case int(offset) >= buf.Len():
		_, err = buf.Write(content)
	default:
		return v2.ResultBadArgument
	}

	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func GetMap(instance common.WasmInstance, mapType v2.MapType) common.HeaderMap {
	ih := getImportHandler(instance)

	switch mapType {
	case v2.MapTypeHttpRequestHeaders:
		return ih.GetHttpRequestHeader()
	case v2.MapTypeHttpRequestTrailers:
		return ih.GetHttpRequestTrailer()
	case v2.MapTypeHttpRequestMetadata:
		return ih.GetHttpRequestMetadata()
	case v2.MapTypeHttpResponseHeaders:
		return ih.GetHttpResponseHeader()
	case v2.MapTypeHttpResponseTrailers:
		return ih.GetHttpResponseTrailer()
	case v2.MapTypeHttpResponseMetadata:
		return ih.GetHttpResponseMetadata()
	case v2.MapTypeHttpCallResponseHeaders:
		return ih.GetHttpCallResponseHeaders()
	case v2.MapTypeHttpCallResponseTrailers:
		return ih.GetHttpCallResponseTrailer()
	case v2.MapTypeHttpCallResponseMetadata:
		return ih.GetHttpCallResponseMetadata()
	default:
		return ih.GetCustomMap(mapType)
	}
}

func copyMapIntoInstance(m common.HeaderMap, instance common.WasmInstance, returnMapData int32, returnMapSize int32) v2.Result {
	cloneMap := make(map[string]string)
	totalBytesLen := u32Len
	m.Range(func(key, value string) bool {
		cloneMap[key] = value
		totalBytesLen += u32Len + u32Len               // keyLen + valueLen
		totalBytesLen += len(key) + 1 + len(value) + 1 // key + \0 + value + \0

		return true
	})

	addr, err := instance.Malloc(int32(totalBytesLen))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	err = instance.PutUint32(addr, uint32(len(cloneMap)))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
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

	err = instance.PutUint32(uint64(returnMapData), uint32(addr))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	err = instance.PutUint32(uint64(returnMapSize), uint32(totalBytesLen))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyGetHeaderMapPairs(ctx context.Context, mapType int32, returnDataPtr int32, returnDataSize int32) int32 {
	instance := h.Instance
	header := GetMap(instance, v2.MapType(mapType))
	if header == nil {
		return int32(v2.ResultNotFound)
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
		return int32(v2.ResultInvalidMemoryAccess)
	}

	err = instance.PutUint32(addr, uint32(len(cloneMap)))
	if err != nil {
		return int32(v2.ResultInvalidMemoryAccess)
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
		return int32(v2.ResultInvalidMemoryAccess)
	}

	err = instance.PutUint32(uint64(returnDataSize), uint32(totalBytesLen))
	if err != nil {
		return int32(v2.ResultInvalidMemoryAccess)
	}

	return int32(v2.ResultOk)
}

func (h *host) ProxyGetHeaderMapValue(
	ctx context.Context, mapType v2.MapType, keyData, keySize, valueData, valueSize int32,
) v2.Result {
	instance := h.Instance
	m := GetMap(instance, mapType)
	if m == nil || keySize == 0 {
		return v2.ResultNotFound
	}

	key, err := instance.GetMemory(uint64(keyData), uint64(keySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(key) == 0 {
		return v2.ResultBadArgument
	}

	value, exists := m.Get(string(key))
	if !exists {
		return v2.ResultNotFound
	}

	return copyIntoInstance(instance, []byte(value), valueData, valueSize)
}

func (h *host) ProxyReplaceHeaderMapValue(
	ctx context.Context, mapType v2.MapType, keyData, keySize, valueData, valueSize int32,
) v2.Result {
	instance := h.Instance
	m := GetMap(instance, mapType)
	if m == nil {
		return v2.ResultNotFound
	}

	key, err := instance.GetMemory(uint64(keyData), uint64(keySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	value, err := instance.GetMemory(uint64(valueData), uint64(valueSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	m.Set(string(key), string(value))

	return v2.ResultOk
}

func (h *host) ProxyOpenSharedKvstore(ctx context.Context, kvstoreNameData int32, kvstoreNameSize int32, createIfNotExist int32,
	returnKvstoreID int32,
) v2.Result {
	instance := h.Instance
	kvstoreName, err := instance.GetMemory(uint64(kvstoreNameData), uint64(kvstoreNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(kvstoreName) == 0 {
		return v2.ResultBadArgument
	}

	callback := getImportHandler(instance)

	kvStoreID, res := callback.OpenSharedKvstore(string(kvstoreName), intToBool(createIfNotExist))
	if res != v2.ResultOk {
		return res
	}

	err = instance.PutUint32(uint64(returnKvstoreID), kvStoreID)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyGetSharedKvstoreKeyValues(ctx context.Context, kvstoreID int32, keyData int32, keySize int32,
	returnValuesData int32, returnValuesSize int32, returnCas int32,
) v2.Result {
	instance := h.Instance
	callback := getImportHandler(instance)

	kvstore := callback.GetSharedKvstore(uint32(kvstoreID))
	if kvstore == nil {
		return v2.ResultBadArgument
	}

	key, err := instance.GetMemory(uint64(keyData), uint64(keySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(key) == 0 {
		return v2.ResultBadArgument
	}

	value, exists := kvstore.Get(string(key))
	if !exists {
		return v2.ResultNotFound
	}

	return copyIntoInstance(instance, []byte(value), returnValuesData, returnValuesSize)
}

func (h *host) ProxySetSharedKvstoreKeyValues(ctx context.Context, kvstoreID int32, keyData int32, keySize int32,
	valuesData int32, valuesSize int32, cas int32,
) v2.Result {
	instance := h.Instance
	callback := getImportHandler(instance)

	kvstore := callback.GetSharedKvstore(uint32(kvstoreID))
	if kvstore == nil {
		return v2.ResultBadArgument
	}

	key, err := instance.GetMemory(uint64(keyData), uint64(keySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(key) == 0 {
		return v2.ResultBadArgument
	}

	value, err := instance.GetMemory(uint64(valuesData), uint64(valuesSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	res := kvstore.SetCAS(string(key), string(value), intToBool(cas))
	if !res {
		return v2.ResultCompareAndSwapMismatch
	}

	return v2.ResultOk
}

func (h *host) ProxyAddSharedKvstoreKeyValues(ctx context.Context, kvstoreID int32, keyData int32, keySize int32,
	valuesData int32, valuesSize int32, cas int32,
) v2.Result {
	instance := h.Instance
	callback := getImportHandler(instance)

	kvstore := callback.GetSharedKvstore(uint32(kvstoreID))
	if kvstore == nil {
		return v2.ResultBadArgument
	}

	key, err := instance.GetMemory(uint64(keyData), uint64(keySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(key) == 0 {
		return v2.ResultBadArgument
	}

	value, err := instance.GetMemory(uint64(valuesData), uint64(valuesSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	res := kvstore.SetCAS(string(key), string(value), intToBool(cas))
	if !res {
		return v2.ResultCompareAndSwapMismatch
	}

	return v2.ResultOk
}

func (h *host) ProxyRemoveSharedKvstoreKey(ctx context.Context, kvstoreID int32, keyData int32, keySize int32, cas int32) v2.Result {
	instance := h.Instance
	callback := getImportHandler(instance)

	kvstore := callback.GetSharedKvstore(uint32(kvstoreID))
	if kvstore == nil {
		return v2.ResultBadArgument
	}

	key, err := instance.GetMemory(uint64(keyData), uint64(keySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(key) == 0 {
		return v2.ResultBadArgument
	}

	res := kvstore.DelCAS(string(key), intToBool(cas))
	if !res {
		return v2.ResultCompareAndSwapMismatch
	}

	return v2.ResultOk
}

func (h *host) ProxyDeleteSharedKvstore(ctx context.Context, kvstoreID int32) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.DeleteSharedKvstore(uint32(kvstoreID))
}

func (h *host) ProxyOpenSharedQueue(ctx context.Context, queueNameData int32, queueNameSize int32, createIfNotExist int32,
	returnQueueID int32,
) v2.Result {
	instance := h.Instance
	queueName, err := instance.GetMemory(uint64(queueNameData), uint64(queueNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(queueName) == 0 {
		return v2.ResultBadArgument
	}

	callback := getImportHandler(instance)

	queueID, res := callback.OpenSharedQueue(string(queueName), intToBool(createIfNotExist))
	if res != v2.ResultOk {
		return res
	}

	err = instance.PutUint32(uint64(returnQueueID), queueID)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyDequeueSharedQueueItem(ctx context.Context, queueID int32, returnPayloadData int32, returnPayloadSize int32) v2.Result {
	instance := h.Instance
	callback := getImportHandler(instance)

	value, res := callback.DequeueSharedQueueItem(uint32(queueID))
	if res != v2.ResultOk {
		return res
	}

	return copyIntoInstance(instance, []byte(value), returnPayloadData, returnPayloadSize)
}

func (h *host) ProxyEnqueueSharedQueueItem(ctx context.Context, queueID int32, payloadData int32, payloadSize int32) v2.Result {
	instance := h.Instance
	value, err := instance.GetMemory(uint64(payloadData), uint64(payloadSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	callback := getImportHandler(instance)

	return callback.EnqueueSharedQueueItem(uint32(queueID), string(value))
}

func (h *host) ProxyDeleteSharedQueue(ctx context.Context, queueID int32) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.DeleteSharedQueue(uint32(queueID))
}

func (h *host) ProxyCreateTimer(ctx context.Context, period int32, oneTime int32, returnTimerID int32) v2.Result {
	instance := h.Instance
	callback := getImportHandler(instance)

	timerID, res := callback.CreateTimer(period, intToBool(oneTime))
	if res != v2.ResultOk {
		return res
	}

	err := instance.PutUint32(uint64(returnTimerID), timerID)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyDeleteTimer(ctx context.Context, timerID int32) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.DeleteTimer(uint32(timerID))
}

func (h *host) ProxyCreateMetric(ctx context.Context, metricType v2.MetricType,
	metricNameData int32, metricNameSize int32, returnMetricID int32,
) v2.Result {
	instance := h.Instance
	ih := getImportHandler(instance)

	name, err := instance.GetMemory(uint64(metricNameData), uint64(metricNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	if len(name) == 0 {
		return v2.ResultBadArgument
	}

	mid, res := ih.CreateMetric(metricType, string(name))
	if res != v2.ResultOk {
		return res
	}

	err = instance.PutUint32(uint64(returnMetricID), mid)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyGetMetricValue(ctx context.Context, metricID int32, returnValue int32) v2.Result {
	instance := h.Instance
	ih := getImportHandler(instance)

	value, res := ih.GetMetricValue(uint32(metricID))
	if res != v2.ResultOk {
		return res
	}

	err := instance.PutUint32(uint64(returnValue), uint32(value))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxySetMetricValue(ctx context.Context, metricID int32, value int64) v2.Result {
	ih := getImportHandler(h.Instance)
	res := ih.SetMetricValue(uint32(metricID), value)

	return res
}

func (h *host) ProxyIncrementMetricValue(ctx context.Context, metricID int32, offset int64) v2.Result {
	ih := getImportHandler(h.Instance)

	return ih.IncrementMetricValue(uint32(metricID), offset)
}

func (h *host) ProxyDeleteMetric(ctx context.Context, metricID int32) v2.Result {
	ih := getImportHandler(h.Instance)

	return ih.DeleteMetric(uint32(metricID))
}

func (h *host) ProxyDispatchHttpCall(ctx context.Context, upstreamNameData int32, upstreamNameSize int32, headersMapData int32, headersMapSize int32,
	bodyData int32, bodySize int32, trailersMapData int32, trailersMapSize int32, timeoutMilliseconds int32,
	returnCalloutID int32,
) v2.Result {
	instance := h.Instance
	upstream, err := instance.GetMemory(uint64(upstreamNameData), uint64(upstreamNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	headerMapData, err := instance.GetMemory(uint64(headersMapData), uint64(headersMapSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	headerMap := common.DecodeMap(headerMapData)

	body, err := instance.GetMemory(uint64(bodyData), uint64(bodySize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	trailerMapData, err := instance.GetMemory(uint64(trailersMapData), uint64(trailersMapSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	trailerMap := common.DecodeMap(trailerMapData)

	ih := getImportHandler(instance)

	calloutID, res := ih.DispatchHttpCall(string(upstream),
		common.CommonHeader(headerMap), common.NewIoBufferBytes(body), common.CommonHeader(trailerMap),
		uint32(timeoutMilliseconds),
	)
	if res != v2.ResultOk {
		return res
	}

	err = instance.PutUint32(uint64(returnCalloutID), calloutID)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyDispatchGrpcCall(ctx context.Context, upstreamNameData int32, upstreamNameSize int32, serviceNameData int32, serviceNameSize int32,
	serviceMethodData int32, serviceMethodSize int32, initialMetadataMapData int32, initialMetadataMapSize int32,
	grpcMessageData int32, grpcMessageSize int32, timeoutMilliseconds int32, returnCalloutID int32,
) v2.Result {
	instance := h.Instance
	upstream, err := instance.GetMemory(uint64(upstreamNameData), uint64(upstreamNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	serviceName, err := instance.GetMemory(uint64(serviceNameData), uint64(serviceNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	serviceMethod, err := instance.GetMemory(uint64(serviceMethodData), uint64(serviceMethodSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	initialMetaMapdata, err := instance.GetMemory(uint64(initialMetadataMapData), uint64(initialMetadataMapSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	initialMetadataMap := common.DecodeMap(initialMetaMapdata)

	msg, err := instance.GetMemory(uint64(grpcMessageData), uint64(grpcMessageSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	ih := getImportHandler(instance)

	calloutID, res := ih.DispatchGrpcCall(string(upstream), string(serviceName), string(serviceMethod),
		common.CommonHeader(initialMetadataMap), common.NewIoBufferBytes(msg), uint32(timeoutMilliseconds))
	if res != v2.ResultOk {
		return res
	}

	err = instance.PutUint32(uint64(returnCalloutID), calloutID)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxyOpenGrpcStream(ctx context.Context, upstreamNameData int32, upstreamNameSize int32, serviceNameData int32, serviceNameSize int32,
	serviceMethodData int32, serviceMethodSize int32, initialMetadataMapData int32, initialMetadataMapSize int32,
	returnCalloutID int32,
) v2.Result {
	instance := h.Instance
	upstream, err := instance.GetMemory(uint64(upstreamNameData), uint64(upstreamNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	serviceName, err := instance.GetMemory(uint64(serviceNameData), uint64(serviceNameSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	serviceMethod, err := instance.GetMemory(uint64(serviceMethodData), uint64(serviceMethodSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	initialMetaMapdata, err := instance.GetMemory(uint64(initialMetadataMapData), uint64(initialMetadataMapSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}
	initialMetadataMap := common.DecodeMap(initialMetaMapdata)

	ih := getImportHandler(instance)

	calloutID, res := ih.OpenGrpcStream(string(upstream), string(serviceName), string(serviceMethod), common.CommonHeader(initialMetadataMap))
	if res != v2.ResultOk {
		return res
	}

	err = instance.PutUint32(uint64(returnCalloutID), calloutID)
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	return v2.ResultOk
}

func (h *host) ProxySendGrpcStreamMessage(ctx context.Context, calloutID int32, grpcMessageData int32, grpcMessageSize int32) v2.Result {
	instance := h.Instance
	grpcMessage, err := instance.GetMemory(uint64(grpcMessageData), uint64(grpcMessageSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	ih := getImportHandler(instance)

	return ih.SendGrpcStreamMessage(uint32(calloutID), common.NewIoBufferBytes(grpcMessage))
}

func (h *host) ProxyCancelGrpcCall(ctx context.Context, calloutID int32) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.CancelGrpcCall(uint32(calloutID))
}

func (h *host) ProxyCloseGrpcCall(ctx context.Context, calloutID int32) v2.Result {
	callback := getImportHandler(h.Instance)

	return callback.CloseGrpcCall(uint32(calloutID))
}

func (h *host) ProxyCallCustomFunction(ctx context.Context, customFunctionID int32, parametersData int32, parametersSize int32,
	returnResultsData int32, returnResultsSize int32,
) v2.Result {
	instance := h.Instance
	param, err := instance.GetMemory(uint64(parametersData), uint64(parametersSize))
	if err != nil {
		return v2.ResultInvalidMemoryAccess
	}

	ih := getImportHandler(instance)

	ret, res := ih.CallCustomFunction(uint32(customFunctionID), string(param))
	if res != v2.ResultOk {
		return res
	}

	return copyIntoInstance(instance, []byte(ret), returnResultsData, returnResultsSize)
}
