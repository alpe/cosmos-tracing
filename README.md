# Cosmos-tracing
Cosmos-tracing is an extension for [Cosmos-SDK](https://github.com/cosmos/cosmos-sdk) based apps to provide thorough insights into the internal execution.
This tool, based on [open tracing](https://opentracing.io/), captures call graphs, performance and additional data.

A modified chain app (known as *Tracing-Node*) can run with the blockchain network as a fullnode to capture data in real time or be synced on old state.  
The [Jaeger](https://www.jaegertracing.io/) provides a store and frontend for traces to visualize them.

## Motivation and history
This extension was developed in parallel to my work at tgrade which has a very complex multi [CosmWasm](https://cosmwasm.com/) 
contract PoE module.
I realized that it would be a nightmare to debug the contract interactions or IO without better tooling. Beside logging and events, 
the blockchain is a black box mostly. One of the major pain points was deciphering error message. Due to non-determinism of error messages, 
there was no way to store them in the original format on chain and to make them available to devs. The Tracing-Node with the Jaeger UI
now solves this problem.

From the initial version that I developed in my own time, some more work was done to make the code easier to adapt to new apps 
and add additional data and traces to capture.[Confio](https://confio.gmbh/) sponsored some of this work, and I am very thankful for this!

Also bit thanks to [Ethan](https://github.com/ethanfrey) was the first power user and provided many good questions and feedback to improve
the data or traces captured and [Pino ](https://github.com/pinosu) who helped with code reviews, integrations and a lot of ops work.


## How to use it
The tracing code needs to be integrated into the blockchain app. This is some manual work where you fork the app and modify the app and command code with tracing extensions.
It is not trivial but this repo includes some examples that should get you started. Find the `tracing` package or diff the code with the original source.
Once you have a patched binary, it is simple:

* First start Jaeger with the following command, for example:
```shell
docker run --name=jaeger -d -p 6831:6831/udp -p 16686:16686 jaegertracing/all-in-one
```
* Then, `start` the Tracing-Node with `--cosmos-tracing.open-tracing` flag set
* Lastly, access UI at [http://localhost:16686](http://localhost:16686)

Once the node starts processing blocks, you'll see traces in the UI.

## Example

```shell
# build a binary
cd examples/wasmd
make build # or build-linux-static

# setup node to connect to a chain

# start with flag
./build/wasmd start --cosmos-tracing.open-tracing --home=<your-nodes-home>
```

## Sponsors
Big thank you also to my sponsor(s) on github, who keep me motivated that Open Source is the way to go. 

## Disclaimer
The provided code is as it is, with no assurances of flawlessness. Users need to be careful during the instrumentation process 
and should backup their data before running a new version with tracing enabled.

Once you have an app-hash error, it is hard to find the root cause. I used to disable specific tracing features until I found the problem.
You can still see feature flags like `os.Getenv("no_tracing_ibcreceive")` in the codebase to support this without re-building a binary.
Especially capturing IBC-results with the Osmosis IBC Hooks enabled creates problems. It is deactivated by default now. (Set ENV `tracing_ibcreceive_capture` to enable) 

## Future versions and support
I am going to provide an example for Cosmos-SDK v0.50 but I can not promise long term support. I have very limited capacity. Also custom support via github issues or discord on individual integrations may not be possible.   

## License & Copyright 
Copyright 2021-2024 Alex Peters
Copyright 2021-2023 Confio GmbH

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0