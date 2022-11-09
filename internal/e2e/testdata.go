package e2e

import (
	"fmt"
	"log"
	"os"

	"github.com/tetratelabs/wabin/binary"
	"github.com/tetratelabs/wabin/wasm"
)

var binAddRequestHeaderV1 = func() []byte {
	return loadWasmWithABI("testdata/add-req-header-v1/main.wasm", "proxy_abi_version_0_1_0")
}()

var binAddRequestHeaderV2 = func() []byte {
	return loadWasmWithABI("testdata/add-req-header-v2/main.wasm", "proxy_abi_version_0_2_0")
}()

func loadWasmWithABI(wasmPath, abiName string) []byte {
	bin, err := os.ReadFile(wasmPath)
	if err != nil {
		log.Panicln(err)
	}
	mod, err := binary.DecodeModule(bin, wasm.CoreFeaturesV2)
	var exports []string
	for _, e := range mod.ExportSection {
		if e.Name == abiName {
			return bin
		}
		exports = append(exports, e.Name)
	}
	log.Panicln(fmt.Errorf("export not found in %s, want: %v, have: %v", wasmPath, abiName, exports))
	return nil
}
