all: diagram.png

diagram.png: diagram.dot
	dot -Tpng $< -o $@
