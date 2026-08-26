// Only the ABI bindings are generated here. The protobuf ones come from `substreams protogen`,
// which is the CLI `substreams build` calls, and are committed under `src/pb`.
fn main() {
    println!("cargo:rerun-if-changed=abi/erc20.json");
    println!("cargo:rerun-if-changed=abi/erc721.json");

    substreams_ethereum::Abigen::new("Erc20", "abi/erc20.json")
        .expect("loading ERC-20 ABI")
        .generate()
        .expect("generating ERC-20 bindings")
        .write_to_file("src/abi/erc20.rs")
        .expect("writing ERC-20 bindings");

    substreams_ethereum::Abigen::new("Erc721", "abi/erc721.json")
        .expect("loading ERC-721 ABI")
        .generate()
        .expect("generating ERC-721 bindings")
        .write_to_file("src/abi/erc721.rs")
        .expect("writing ERC-721 bindings");
}
