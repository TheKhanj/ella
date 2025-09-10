if [ -z "$_INC_COMMON" ]; then
	_INC_COMMON=1

	get_latest_version() {
		curl -s https://api.github.com/repos/thekhanj/ella/releases/latest |
			grep '"tag_name":' |
			sed -E 's/.*"v([^"]+)".*/\1/'
	}

	get_checksums() {
		local pkgver="$1"

		local file="ella_sha256_checksums_$pkgver.txt"

		if ! [ -f "$file" ]; then
			curl -sL -o "$file" \
				"https://github.com/thekhanj/ella/releases/download/v${pkgver}/ella_sha256_checksums.txt"
		fi

		cat "$file"
	}

	get_checksum() {
		local pkgver="$1"
		local binname="$2"

		get_checksums "$pkgver" |
			grep "$binname" |
			awk '{ print $1 }'
	}

	get_binnames() {
		local pkgver="$1"

		# don't change the order
		echo "ella_v${pkgver}_linux_amd64.tar.gz"
		echo "ella_v${pkgver}_linux_arm64.tar.gz"
		echo "ella_v${pkgver}_linux_arm_hf.tar.gz"
		echo "ella_v${pkgver}_linux_arm.tar.gz"
		echo "ella_v${pkgver}_linux_loong64.tar.gz"
		echo "ella_v${pkgver}_linux_mips.tar.gz"
		echo "ella_v${pkgver}_linux_mips64.tar.gz"
		echo "ella_v${pkgver}_linux_mips64le.tar.gz"
		echo "ella_v${pkgver}_linux_mipsle.tar.gz"
		echo "ella_v${pkgver}_linux_riscv64.tar.gz"
	}
fi
