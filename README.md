# Roo

Bouncy bouncy bounce

## KV Store

```
com.roo.host:<requesting_host:port>  <destination_host:port>
```

## API 
### Post a key to the distributed store
```
curl -X PUT -d'test data' http://localhost:6299/roo/v1/kv/test
```
* Returns a json object { "ok" : true} if succeded
#### A Real Example
**Put in a test route**. For our example we are serving goole.com at our public endpoint, and routing to a swarm service with an endpoint inside our network on port 9001. The swarm stack is _cool_, the swarm service name is _game_. In docker swarm world this equates to a mesh network load balanced DNS record of tasks.cool_game. (you'll need to find yours to get it working). The overall result to add this to our routing tables is:

```curl -X PUT -d'http://tasks.cool_game:9001' http://localhost:6299/roo/v1/kv/com.roo.host:google.com:443```

To test you can run something like this (this just makes your localhost pretend like its responding to a request to google.com):
```curl -H "Host: google.com" https://localhost:443/```

### Get a key from the store
```
curl -X GET http://localhost:6299/roo/v1/kv/test
```
* Returns the raw bytes
### Query the store (SCAN query)
```
curl -X GET http://localhost:6299/roo/v1/kvs/te #Searches prefix te
curl -X GET http://localhost:6299/roo/v1/kvs #Gets everything
```
* Use ```window.atob("dGVzdCBkYXRh")``` in javascript. Use json.Unmarshall or string([]byte) in Golang Go if you want a string.

## TODO

[ ] Autoscale Docker
[ ] Autoscale Physical Infratructure
[ ] Move flaoting IPs (Load balance, service down)
[ ] SSL in API
[ ] HTTP for Proxying (Only SSL Supported atm)
[ ] Auto downgrade 

## Credits
* [DragonBoat](https://github.com/lni/dragonboat)
* [DragonGate](https://github.com/dioptre/DragonGate)
* [SF Product Labs](https://sfproductlabs.com)
