VERSION = $(shell . inc/common.sh; get_latest_version)

all: .SRCINFO PKGBUILD

.SRCINFO: tmpl/.SRCINFO
	@./bin/gensrcinfo $(VERSION) $< > $@

tmpl/.SRCINFO: PKGBUILD
	makepkg --printsrcinfo >$@

PKGBUILD: tmpl/PKGBUILD
	@./bin/genpkgbuild $(VERSION) $< >$@
