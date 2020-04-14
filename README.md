# Roo

Bouncy bouncy bounce

## KV Store

```
com.roo.host:<host>  <host:port>
```

## API 
### Post a key to the distributed store
```
curl -X PUT -d'test data' http://localhost:6299/roo/v1/kv/test
```
### Get a key from the store
```
curl -X GET http://localhost:6299/roo/v1/kv/test
```
### Query the store (SCAN query)
```
curl -X GET http://localhost:6299/roo/v1/kvs/te
```
* Use ```window.atob("dGVzdCBkYXRh")``` in javascript or just json.Unmarshall with []byte in Golang Go

## TODO
```
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
