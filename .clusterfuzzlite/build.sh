#!/bin/bash -eu

cd "$SRC/github.com/traweezy/stackctl"

compile_go_fuzzer github.com/traweezy/stackctl/internal/output FuzzRenderMarkdownGo markdown_render gofuzz
compile_go_fuzzer github.com/traweezy/stackctl/internal/config FuzzConfigLoadAndRenderGo config_load_render gofuzz
