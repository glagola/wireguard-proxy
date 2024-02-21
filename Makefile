.PHONY: all clean

all: 

run:
	go run ./main.go

run_server:
	nc -ul 1000

run_client:
	nc -u localhost 51820

clean:
	rm -rf ./main
