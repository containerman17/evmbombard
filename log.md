# First attempt - same insane gas limit, 250ms block time


```
sudo docker stop avago; 
sudo docker rm avago; 
sudo docker run -it -d \
    --name avago \
    --network host \
    -v ~/.avalanchego:/root/.avalanchego \
    -e AVAGO_PARTIAL_SYNC_PRIMARY_NETWORK=true \
    -e AVAGO_PUBLIC_IP_RESOLUTION_SERVICE=opendns \
    -e AVAGO_HTTP_HOST=0.0.0.0 \
    -e AVAGO_TRACK_SUBNETS=2qvFnYKCgfy3w6pm8HTCXLChf7xJpe3mn5mTDb6bpzy3qSB1ji \
    -e AVAGO_NETWORK_ID=fuji \
    -e AVAGO_HTTP_ALLOWED_HOSTS="*" \
    -e AVAGO_PROPOSERVM_MIN_BLOCK_DELAY="900ms" \
    martineck/subsecond-blocktime
```

Ok, let's try running the benchmark:

```
go run . -rpc "http://127.0.0.1:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc,http://43.206.117.145:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc,http://13.115.252.62:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc,http://18.183.84.49:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc,http://18.183.103.247:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc,http://35.77.217.46:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc," -tps 1000 -keys 10000

```

```
go run . -rpc "http://127.0.0.1:9650/ext/bc/QarMfHatVYh64GV7fmoSk8YNfaTyMUBJZ5gUF4P2axJTzfta2/rpc" -tps 10 -keys 1000

```
