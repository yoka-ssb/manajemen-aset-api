# [Asset Management API]
================

[ This is a provided API for Asset Management App. Generated with GRPC in server and REST in client ]

## Table of Contents
-----------------

* [Getting Started](#getting-started)
* [Features](#features)
* [Installation](#installation)
* [Usage](#usage)
* [Contributing](#contributing)
* [License](#license)

## Getting Started
---------------

### Install gRPC library
pip install grpc

### Install Protocol Buffers compiler
pip install protobuf 

## Features
--------

## Installation
------------

[ Instructions on how to install the project, including any dependencies or setup required ]

## Usage
-----

### Generate protofile

Run this code to generate protobuf:
protoc -I . -I ./googleapis \                                                                                  
  --go_out ./assetpb --go_opt paths=source_relative \
  --go-grpc_out ./assetpb --go-grpc_opt paths=source_relative \
  --grpc-gateway_out ./assetpb --grpc-gateway_opt paths=source_relative \
  asset.proto

### Hit REST API
Here is the example curl:
curl -X POST \
  http://localhost:50051/myservice/mymethod \
  -H 'Content-Type: application/json' \
  -d '{"name": "World"}'

Or we can use postman.

## Contributing
------------

## License
-------
