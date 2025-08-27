CONFIG_GO_SRC_FILES = $(shell find ./config -name '*.go') config/config.go

all: service.png

service.png: service.dot
	dot -Tpng $< -o $@

config: $(CONFIG_GO_SRC_FILES)

config/config: schema.json
	cat $< > $@

config/%.go: config/%
	go-jsonschema -p config $< > $@
