VERSION = $(shell git describe --tags --exact-match 2>/dev/null || echo -n dev)
LD_FLAGS = -X 'main.VERSION=$(VERSION)'
ifneq ($(VERSION),dev)
LD_FLAGS += -s -w
endif

CONFIG_GO_SRC_FILES = config/config.go $(shell find ./config -name '*.go')
FSM_DOT_FILES = $(shell find ./fsm -name '*.dot')
FSM_PNG_FILES = $(patsubst %.dot,%.png,$(FSM_DOT_FILES))

GO_SRC_FILES = $(shell find . -name '*.go' | grep -v '^./config')

all: ella man $(FSM_PNG_FILES)

clean:
	rm ella fsm/*.png config/config.go

install: all
	@./install

man: man/ella.1.gz
	@touch man

man/%.gz: man/%.roff
	gzip -9 -c $< > $@

ella: $(GO_SRC_FILES) $(CONFIG_GO_SRC_FILES) .version
	go generate && \
		go build \
			-ldflags "$(LD_FLAGS)" \
			-o $@

fsm/%.png: fsm/%.dot
	dot -Tpng $< -o $@

config/config: schema.json
	cat $< > $@

config/%.go: config/%
	go-jsonschema -p config $< > $@

assert-version:
	@if ! [ -f .version ] || \
		[ "$(shell cat .version 2>&1)" != "$(VERSION)" ]; then \
		echo $(VERSION) > .version; \
	fi

# keep this rule's command there, it's mandatory, I don't know why :)
.version: assert-version
	@true >/dev/null

.PHONY: assert-version clean
