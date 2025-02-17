# Protocol Buffers (Protos) Directory Structure & Best Practices

## Motivation

The `protos` directory is structured to facilitate **data modeling** in a way that allows **cross-language compatibility** while maintaining a well-defined format for the **Stellar ecosystem** and downstream clients.

By defining clear `.proto` specifications, we ensure:
- **Cross-Language Code Generation**: The same `.proto` files can be used to generate client libraries in **Go, Python, Java, Rust**, etc.
- **Consistent Data Representation**: All downstream consumers (including Stellar-based applications) receive structured data in a well-defined format.
- **Separation of Concerns**: The `.proto` files define **data models** independently of any specific implementation logic.
- **Efficient Serialization**: Protocol Buffers are lightweight and optimized for **fast serialization/deserialization**, making them ideal for large-scale data ingestion.

To maximize these benefits, the `protos` directory is carefully organized to ensure clarity and maintainability.

## Directory Structure

The `protos` directory follows a structured layout to ensure maintainability and clarity. The structure looks like this:

```
protos/
├── ingest/
│   ├── asset/
│   │   ├── asset.proto
│   ├── address/
│   │   ├── address.proto
│   ├── processors/
│       ├── token_transfer/
│           ├── token_transfer_event.proto
```

## Code Generation

The generated Go code appears in the corresponding locations with the `protos/` prefix stripped.
For example, the generated code for:

`protos/ingest/processors/token_transfer/token_transfer_event.proto`

will be generated under:

`ingest/processors/token_transfer/token_transfer_event.pb.go`

This structure is maintained to ensure Go packages align correctly with their corresponding `.proto` files.

## Import Paths
- The top-level include path for .proto files is `protos/`. 
- All imports inside `.proto` files must be relative to the top-level `protos/` directory.
- For e.g, in `protos/ingest/processors/token_transfer/token_transfer_event.proto`:
    ```
    syntax = "proto3";
    
    package token_transfer;
    
    import "google/protobuf/timestamp.proto";
    import "ingest/address/address.proto";
    import "ingest/asset/asset.proto";
    ```
    Here, `address.proto` and `asset.proto` are located at: `protos/ingest/address/address.proto` and `protos/ingest/asset/asset.proto`


## Best Practices

- **Follow Directory Structure**
  - Organize `.proto` files into directories that match logical groupings.
  - Keep related `.proto` files together (e.g., All data models related to token_tranfer should be under `protos/ingest/processors/token_transfer/`).


- **Consistent Package Naming**
  - The `package` name inside each `.proto` file should match the directory structure under which it appers.
  - For e.f, the package name for  `protos/ingest/processor/token_transfer_event.proto`  should be:
     ```
     syntax = "proto3";
     package token_transfer;
     ```
    
- **Generated Files**
  - Never edit the generated `.pb.go` files.
  - The go bindings from the `.proto` files are generated from the [top level Makefile](./../Makefile)
    ```
    make generate-proto
    ```
    or
    ```
    make regenerate-proto
    ```
    _Always invoke these commands from the top-level directory_.


- **Helper Functions**
  - Any helper functions related to the generated protobuf structures should be placed in the same directory as the generated `.pb.go` files.
  - Ensure they use the same package name to maintain compatibility.


- **Checking in generateds**
    - Generated .pb.go files are committed to the repository.
    - This ensures that clients using the package do not need to regenerate the bindings themselves when using the `ingest` package.
