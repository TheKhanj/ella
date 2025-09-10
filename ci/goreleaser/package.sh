#!/usr/bin/env sh

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

set -e

make
if [ $? -ne 0 ]; then
	echo "make error"
	exit 1
fi

ella_version=$(./ella -v)
echo "build version: $ella_version"

# cross_compiles
make -f ./ci/goreleaser/Makefile

rm -rf ./release/packages
mkdir -p ./release/packages

os_all='linux windows darwin freebsd openbsd android'
arch_all='386 amd64 arm arm64 mips64 mips64le mips mipsle riscv64 loong64'
extra_all='_ hf'

cd ./release

for os in $os_all; do
	for arch in $arch_all; do
		for extra in $extra_all; do
			suffix="${os}_${arch}"
			if [ "x${extra}" != x"_" ]; then
				suffix="${os}_${arch}_${extra}"
			fi
			ella_dir_name="ella_${ella_version}_${suffix}"
			ella_path="./packages/ella_${ella_version}_${suffix}"

			if [ "x${os}" = x"windows" ]; then
				if [ ! -f "./ella_${os}_${arch}.exe" ]; then
					continue
				fi
				mkdir ${ella_path}
				mkdir ${ella_path}/fsm
				mkdir ${ella_path}/man
				mv ./ella_${os}_${arch}.exe ${ella_path}/ella.exe
			else
				if [ ! -f "./ella_${suffix}" ]; then
					continue
				fi
				mkdir ${ella_path}
				mkdir ${ella_path}/fsm
				mkdir ${ella_path}/man
				mv ./ella_${suffix} ${ella_path}/ella
			fi
			cp ../install ${ella_path}
			cp ../completion.sh ${ella_path}
			cp ../man/ella.1.gz ${ella_path}/man/ella.1.gz
			cp ../fsm/service.png ${ella_path}/fsm

			# packages
			cd ./packages
			if [ "x${os}" = x"windows" ]; then
				zip -rq ${ella_dir_name}.zip ${ella_dir_name}
			else
				tar -zcf ${ella_dir_name}.tar.gz ${ella_dir_name}
			fi
			cd ..
			rm -rf ${ella_path}
		done
	done
done

cd -
