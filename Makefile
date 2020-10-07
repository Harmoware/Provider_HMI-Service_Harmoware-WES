GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
RM=rm


TARGET=hmi-service
# Main target

all: $(TARGET)

.PHONY: build 
build: $(TARGET)

$(TARGET): 
	$(GOBUILD) $(TARGET).go

.PHONY: clean
clean: 
	$(RM) $(TARGET)

.PHONY: run
run:
	./$(TARGET)