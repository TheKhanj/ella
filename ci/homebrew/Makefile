VERSION = $(shell . inc/common.sh; get_latest_version)

all: Formula/ella.rb

Formula/ella.rb: tmpl/ella.rb
	@./bin/genellarb $(VERSION) $< >$@
