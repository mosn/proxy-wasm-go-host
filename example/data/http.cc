// Copyright 2016-2020 Envoy Project Authors
// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <string>
#include <string_view>
#include <stdlib.h>

#include "proxy_wasm_intrinsics.h"

class ExampleContext : public Context {
public:
  explicit ExampleContext(uint32_t id, RootContext *root) : Context(id, root) {}

  FilterHeadersStatus onRequestHeaders(uint32_t headers, bool end_of_stream) override;
};

static RegisterContextFactory register_ExampleContext(CONTEXT_FACTORY(ExampleContext));


FilterHeadersStatus ExampleContext::onRequestHeaders(uint32_t, bool) {
  LOG_DEBUG(std::string("print from wasm, onRequestHeaders, context id: ") + std::to_string(id()));

  auto result = getRequestHeaderPairs();
  auto pairs = result->pairs();
  for (auto &p : pairs) {
    LOG_INFO(std::string("print from wasm, ") + std::string(p.first) + std::string(" -> ") + std::string(p.second));
  }

  return FilterHeadersStatus::Continue;
}
