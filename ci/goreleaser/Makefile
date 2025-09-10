# -----------------------------------------------------------------------------
# Portions of this file are Copyright (c) 2015-2025 fatedier <fatedier@gmail.com>
# Original work licensed under the Apache License 2.0
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Modifications (c) 2025 Pooyan
# Licensed under the MIT License
#
# You may obtain a copy of the MIT License at:
#     https://opensource.org/licenses/MIT
#
# Description:
# This file is based on FRP (https://github.com/fatedier/frp) and has been
# modified for use in my project.
# -----------------------------------------------------------------------------

VERSION = $(shell git describe --tags --exact-match 2>/dev/null || echo -n dev)
LD_FLAGS = -X 'main.VERSION=$(VERSION)'
ifneq ($(VERSION),dev)
LD_FLAGS += -s -w
endif

# TODO: not supported yet
# windows:amd64 windows:arm64
os-archs=darwin:amd64 darwin:arm64 freebsd:amd64 openbsd:amd64 \
	linux:amd64 linux:arm:7 linux:arm:5 linux:arm64 \
	linux:mips64 linux:mips64le linux:mips:softfloat \
	linux:mipsle:softfloat linux:riscv64 linux:loong64 android:arm64

all:
	@$(foreach n, $(os-archs), \
		os=$(shell echo "$(n)" | cut -d : -f 1); \
		arch=$(shell echo "$(n)" | cut -d : -f 2); \
		extra=$(shell echo "$(n)" | cut -d : -f 3); \
		flags=''; \
		target_suffix=$${os}_$${arch}; \
		if [ "$${os}" = "linux" ] && [ "$${arch}" = "arm" ] && [ "$${extra}" != "" ] ; then \
			if [ "$${extra}" = "7" ]; then \
				flags=GOARM=7; \
				target_suffix=$${os}_arm_hf; \
			elif [ "$${extra}" = "5" ]; then \
				flags=GOARM=5; \
				target_suffix=$${os}_arm; \
			fi; \
		elif [ "$${os}" = "linux" ] && ([ "$${arch}" = "mips" ] || [ "$${arch}" = "mipsle" ]) && [ "$${extra}" != "" ] ; then \
		    flags=GOMIPS=$${extra}; \
		fi; \
		echo "Build $${os}-$${arch}$${extra:+ ($${extra})}..."; \
		env GOOS=$${os} GOARCH=$${arch} $${flags} go build -trimpath -ldflags "$(LD_FLAGS)" -o ./release/ella_$${target_suffix} .; \
		echo "Build $${os}-$${arch}$${extra:+ ($${extra})} done"; \
	)
