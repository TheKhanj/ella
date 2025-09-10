if [ -z "$_INC_COMMON" ]; then
	_INC_COMMON=1

	get_latest_version() {
		curl -s https://api.github.com/repos/thekhanj/ella/releases/latest |
			grep '"tag_name":' |
			sed -E 's/.*"v([^"]+)".*/\1/'
	}

	get_checksums() {
		echo "$pkgver"
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
fi
