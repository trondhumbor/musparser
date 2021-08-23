build-dir:
	mkdir -p bin

build: build-dir
	go build -o bin/musparser cmd/main/main.go

run: # make ARGS="-arg1 val1 -arg2 -arg3" run
	./bin/musparser ${ARGS}

clean:
	rm -r bin
