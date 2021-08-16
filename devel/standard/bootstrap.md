## Dev1 Bootstrap Log (Tue Apr  6 12:35:14 EDT 2021)

```
$ /Users/maoueh/work/dfuse/ethereum.battlefield/node_modules/.bin/ts-node src/main.ts
Configuration
 Network: local
 Default address: 0x821b55d8abe79bc98f05eb675fdc50dfe796b7ab
 RPC Endpoint: http://localhost:8545

Deploying contracts...
Deploying contract 'UniswapV2Factory'
Deploying contract 'GrandChild'
Deploying contract 'Suicidal'
Deploying contract 'Child'
Deploying contract 'Suicidal'
Deploying contract 'Main'
Deploying contract 'EIP20Factory'

Contracts
- main => 0xCae819bff9B75c3D34971C19e005F2cAD7354E0f
- child => 0x929bc44BBD41Ca0e621dc50f7c7E3204Ce026258
- grandChild => 0x702641c70a11E480F646Ed247d078c65aBAAC5DE
- suicidal1 => 0xEC9C1fCee156bf34Ba4fB5D38C9CF09Df16723eF
- suicidal2 => 0x9a77F7b94488d24EcA50FA0d144212AE48300A71
- uniswap => 0x71940c77ccadaeA1238CEa27674E6253128ca177
- erc20 => 0x314F9285cbC3835e676974abDb7d2ab45ede3686

Transaction Links
- main => https://localhost:8080/tx/0xeee3dd4bb8728db085ca57bb85f5ee18bc1a467e3ec75fc0b4105cc270bd5796
- child => https://localhost:8080/tx/0x27e234af8a8c915dc10c934ca6619e09d0411d7b9e927315ade9f475013f42f1
- grandChild => https://localhost:8080/tx/0xfeb7158587ff596ed583fcf3f935e627b2b0ac3c03b99f5b12f1d577ad8f57c4
- suicidal1 => https://localhost:8080/tx/0xdb817b0906b80b24654db5951f054aa42b2ff282bf29f0a4c37b38af6f008236
- suicidal2 => https://localhost:8080/tx/0x031c0bcbc9a78d878f2587fff81724b1e834e8f3a60d6a5941c2a6d39f3beac4
- uniswap => https://localhost:8080/tx/0xbda611d85e6df66b4b2fe2d4edc8d518068df19bdcd1c317bda7988a7aecccb4
- erc20 => https://localhost:8080/tx/0x41b56fc456afb55331cac0566e4fdeddf8bc8e7277173b648032bdb91cf55bb9

Performing pure 'transfer' transactions
Pushed transaction 'https://localhost:8080/tx/0x5a215c52d4626649d12dd5ba78d6c04ab43e80d6a25e61f50d091562c2b06ad3' (pure transfer: existing address)
Pushed transaction 'https://localhost:8080/tx/0xec6ab9a187d19e7d20c80508440f9cebdb308decf08a336e7e43b66298af433c' (pure transfer: existing address with custom gas limit & price)
Pushed transaction 'https://localhost:8080/tx/0x3558864e9075d94a8577a69123bee4cfd0a48dbc162b7a9258aa0ef004c2874f' (pure transfer: inexistant address creates account and has an EVM call)
Pushed transaction 'https://localhost:8080/tx/0xbcb126cbe64f2286ba3a7ad4b30d0744c58a9dff21f38a2f1e59b0ea9867d0c3' (pure transfer: transfer of 0 ETH to inexistant address generates a transaction with no EVM call)

Performing 'transfer' through contract transactions
Pushed transaction 'https://localhost:8080/tx/0x89d172b5866cfa05a6c6dbc6f4587323a2f1342827d35a62383cb2e610820ea3' (nested transfer through contract: existing addresss)
Pushed transaction 'https://localhost:8080/tx/0x656a5158873ae03b2bd132bdc322d032abdcc8520e17f7485276f4d9c90b47dd' (transfer through contract: existing addresss)
Pushed transaction 'https://localhost:8080/tx/0x47f09a08c3fff93c48c141ed4d3b6199802fa7f051104922ecd18ef1884f7412' (transfer through contract: inexistant address creates account and has an EVM call)
Pushed transaction 'https://localhost:8080/tx/0xad2450f84f9d9e547490b41bd26c3920bdfc3ac2a179bb1a454e58a30b1801cb' (nested transfer through contract: inexistant address creates account and has an EVM call)

Performing failing 'transfer' through contract transactions
Pushed transaction 'https://localhost:8080/tx/0x286e35168fa508e6541918467b48b6d1ef92676d406067c51556348609b6eb93' (transfer through contract: existing addresss correctly failed with EVM reverted without reason)

Performing 'log' transactions
Pushed transaction 'https://localhost:8080/tx/0x29ed9a5610fa22a02e76ab11995a755d7d6dcda8f21cbe0120d50a9fee9b5705' (log: all)
Pushed transaction 'https://localhost:8080/tx/0xffddf4d8ccfa66767bfe2dc00d46a744ff6dc9d6d6bf54526c32de979266ed46' (log: empty)
Pushed transaction 'https://localhost:8080/tx/0x62e88e75f275a03e57a1d149d67a7708c074e1385ac0d6f36012983ff724acf8' (log: single)
Pushed transaction 'https://localhost:8080/tx/0x53f91af412732884437364b455f8353eb44152d5dedbd60e3ef3472c05c16c6c' (log: multi)
Pushed transaction 'https://localhost:8080/tx/0x9a14092f7bce81fc5d9f16c1119f53b98f04bd8a5c30938add9352b033cc12b0' (log: all mixed)
Pushed transaction 'https://localhost:8080/tx/0x2374506f96d4fa85abf8edd05c9eddec064b9a1f519cbb8cb3351d7c5b6d5111' (log: all indexed)

Performing 'storage & input' transactions
Pushed transaction 'https://localhost:8080/tx/0xae4000ea075f9c72a092a2a9706674d6b6f8ff0a9206159ca63baf58f15453fa' (input: string equal 0)
Pushed transaction 'https://localhost:8080/tx/0x1ed13d560c0c75aa88c2bff668b91d9fcb238c1f3784bfd166f612cca8729bc8' (input: string equal 30)
Pushed transaction 'https://localhost:8080/tx/0x0126a564539f25a221a668162ff62bacbbff0d26f1d75643d3f22ff324730f16' (input: string equal 15)
Pushed transaction 'https://localhost:8080/tx/0x910b0aaa6545b5b87e64bb9d7b4d71ea3aafc543332394376bcb9b0bee8228cf' (input: string equal 31)
Pushed transaction 'https://localhost:8080/tx/0x13fbc182c98993e60b151a82f06f72bae9e67c9bd76746b4c5ef03d981e5ac65' (storage: set long string)
Pushed transaction 'https://localhost:8080/tx/0x1674c607b5f17d945b5c92696c418fede69157a3f74801d2314344b5b1fd78f0' (input: string longer than 32)
Pushed transaction 'https://localhost:8080/tx/0xc8353067300deaa8d6a99b896e3cc757e0c28e6badbf57914296a9adad4dffac' (input: string equal 32)
Pushed transaction 'https://localhost:8080/tx/0x4b7ec72c38a3cf7c970007ba2e3b17ec2e043c7a34f74e8011db2586541710b0' (storage: array update)

Performing 'call' & 'constructor' transactions
Pushed transaction 'https://localhost:8080/tx/0xaa36f5815079ebf20da5a7602c6e51d66169fc96c5aadd71abe9fcc142931fc6' (call: nested call revert state changes)
Pushed transaction 'https://localhost:8080/tx/0x7499dff5f4fa7163acb61f051908b512fa9dac287c5299b42fa72df3f272ba60' (call: contract creation from call, with constructor)
Pushed transaction 'https://localhost:8080/tx/0x293fa17feb024205c48419c6362aadde8fa60a252bf454057a380613ac99a72d' (call: contract creation from call, recursive constructor, second will fail correctly failed with EVM reverted without reason)
Pushed transaction 'https://localhost:8080/tx/0x5e004658f888ebabfb3db69f1425f9aec83576bbc0ee37ffca4a1149755aedd0' (call: contract creation from call, with constructor that will fail correctly failed with EVM reverted without reason)
Pushed transaction 'https://localhost:8080/tx/0x37b03185866e8fd1442848c1f7c5cc44009aa33486652b7b0e380fcde7f253df' (call: all pre-compiled)
Pushed transaction 'https://localhost:8080/tx/0x079db4d6a2ff3ba54560c1dda350a22f94c92d65d16a1f7572bf009934f65696' (call: revert failure root call correctly failed with EVM reverted without reason)
Pushed transaction 'https://localhost:8080/tx/0x26559f7e7db6be409c59250fbc089a8d6a411231906daaefcf47cc37792e40dd' (call: complete call tree)
Pushed transaction 'https://localhost:8080/tx/0xf9e326c37a9cf03ddad4f192d244d2a40ec53767e2c91335c1e5633564f1a47d' (call: nested fail with native transfer)
Pushed transaction 'https://localhost:8080/tx/0x42d7249dcafe9b8ca62c372dacb42bf844966bc95e5f92f2d488be83fa4a0ec5' (call: contract creation from call, without a constructor)
Pushed transaction 'https://localhost:8080/tx/0x1e247b520f49343173f53d4df399ff65a7334bde116eba0ae2d16af20d4e55a6' (call: contract with create2, succesful creation)
Pushed transaction 'https://localhost:8080/tx/0xb6a7e281deeacc0d0c9fd1d4f2285813c1d1ef401c6cbc6860a0e6750818cc66' (call: contract with create2, inner call fail due to insufficent funds (transaction succeed though))
Pushed transaction 'https://localhost:8080/tx/0x0323420a7e3ed35bed5a7b89211548ab6e47b6f5aeddc340c1bd708cd106697c' (call: contract with create2, inner call fail due to insufficent funds then revert correctly failed with EVM reverted without reason)
Pushed transaction 'https://localhost:8080/tx/0x7ed7333c2067e3cfaa218ef8dee79a0b35681754c2cd004ace2be554e7c2edca' (call: assert failure root call correctly failed with EVM reverted without reason)
Pushed transaction 'https://localhost:8080/tx/0x5d164af5354980219ff1fd72e6e438f08fd5beefc3a1c2ab369dda481b7cbabe' (call: assert failure on child call correctly failed with EVM reverted without reason)
Pushed transaction 'https://localhost:8080/tx/0x10d3fad75da963b8da4f27f9d425075cccafb3d83f11baad5446d542c1760c42' (call: contract fail not enough gas after code_copy correctly failed with The contract code couldn't be stored, please check your gas limit.)
Pushed transaction 'https://localhost:8080/tx/0x9badf7cb7ab4295c0a840f6ef084bf1e5051e5ffcc020090cd5e9d2b49a6ae70' (call: contract fail just enough gas for intrinsic gas correctly failed with The contract code couldn't be stored, please check your gas limit.)
Pushed transaction 'https://localhost:8080/tx/0xf6468214638e75c58746db1a8867a64194bcf67e8958193371748ab00543e288' (call: contract with create2, inner call fail due to address already exists (transaction succeed though))
Pushed transaction 'https://localhost:8080/tx/0xcb25e752283059ce8156e95d3b1bf703824e18bff5534405bb11c08ba5473fae' (call: contract with create2, inner call fail due to address already exists then revert correctly failed with EVM reverted without reason)

Performing 'gas' transactions
Pushed transaction 'https://localhost:8080/tx/0x12af95f94b924f19ea9825ac9c315edeedf507e5a4576532fedfa5e68f3edefe' (gas: empty call for lowest gas)
Pushed transaction 'https://localhost:8080/tx/0x54e4d6c4f9d4bb9bfbd0fdba8627b1190e24623e6076f3c038c598159d7e4998' (gas: deep nested nested call for lowest gas)
Pushed transaction 'https://localhost:8080/tx/0x31b2ff1440cf87a077236130ac9556bf31062666c0dd9876ff8f50205042adc2' (gas: nested low gas)
Pushed transaction 'https://localhost:8080/tx/0x2fb463113566966e70abf32b2de029dcfe81b76e8f5e525d4188ee6caa7c712f' (gas: deep nested low gas)
Pushed transaction 'https://localhost:8080/tx/0xa3d35bb10c45ef5d1288b076e49c656350e18604f93fd0f850a3d8b511533f67' (gas: deep nested call for lowest gas)

Performing 'suicide' transactions
Pushed transaction 'https://localhost:8080/tx/0x3e8bae9ed05a3a699a3dc25f59041ac7adf9b6ecb991d78c16a530e61d63c556' (suicide: transfer some Ether to contract that's about to suicide itself)
Pushed transaction 'https://localhost:8080/tx/0x1886b68a957b9de56e1f8e573a85900486e80653641ea65e1d7a43b4e1871f3f' (suicide: contract does not hold any Ether)
Pushed transaction 'https://localhost:8080/tx/0x06d0fa237690354b582e2a4b29428cb6924e4b63ba9f9f3dfbc9b66cf4da1950' (suicide: contract does hold some Ether, and refund owner on destruct)

Completed battlefield deployment (local)
```
