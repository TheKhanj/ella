CONFIG_GO_SRC_FILES = $(shell find ./config -name '*.go') config/config.go
FSM_DOT_FILES = $(shell find ./fsm -name '*.dot')
FSM_PNG_FILES = $(patsubst %.dot,%.png,$(FSM_DOT_FILES))

all: $(FSM_PNG_FILES)

fsm/%.png: fsm/%.dot
	dot -Tpng $< -o $@

config: $(CONFIG_GO_SRC_FILES)

config/config: schema.json
	cat $< > $@

config/%.go: config/%
	go-jsonschema -p config $< > $@
