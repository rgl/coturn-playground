# About

This is a [coturn](https://github.com/coturn/coturn) playground for playing with [Session Traversal Utilities for NAT (STUN)](https://en.wikipedia.org/wiki/STUN) and [Traversal Using Relays around NAT (TURN)](https://en.wikipedia.org/wiki/Traversal_Using_Relays_around_NAT).

# Usage

Edit the `turnserver.conf` IP address and use it through this example.

**NB** This example assumes `10.3.0.1`.

Start the `coturn` server:

```bash
docker compose up --build
```

**NB** You might need to [configure the firewall](#iptables-rules).

In another shell, initialize it:

```bash
docker compose run coturn sqlite3 /var/lib/coturn/turndb .schema
docker compose run coturn turnadmin --add-admin --realm coturn --user admin --password admin
docker compose run coturn turnadmin --add --realm coturn --user alice --password alice
docker compose run coturn turnadmin --list-admin
docker compose run coturn turnadmin --list
```

And try it:

```bash
cd turn-ping
docker build --tag turn-ping .
docker run --rm turn-ping -host 10.3.0.1 -port 3478 -realm coturn -user alice=alice -protocol tcp
docker run --rm turn-ping -host 10.3.0.1 -port 3478 -realm coturn -user alice=alice -protocol udp
```

Also try it with the [Trickle ICE WebRTC sample](https://webrtc.github.io/samples/src/content/peerconnection/trickle-ice/):

* STUN or TURN URI: `turn:10.3.0.1:3478`
* TURN username: `alice`
* TURN password: `alice`
* IceTransports value: `relay`

The setup is working when you see a `rtp relay` line.

# iptables rules

Edit the saved rules:

```bash
vim /etc/iptables/rules.v4
```

Add the required rules:

```
-A INPUT -p tcp -m state --state NEW -m multiport --dports 3478:3479 -j ACCEPT
-A INPUT -p udp -m multiport                      --dports 3478:3479 -j ACCEPT
-A INPUT -p tcp -m state --state NEW -m multiport --dports 49160:49200 -j ACCEPT
-A INPUT -p udp -m multiport                      --dports 49160:49200 -j ACCEPT
```

Reboot to apply:

**NB** We reboot because we are also using docker, which dynamically creates iptables rules, and since we do not want to save those, we cannot just do a `iptables-restore /etc/iptables/rules.v4`.

```bash
reboot
```

# Notes

* Instead of configuring all the users in the coturn server using `lt-cred-mech`, you might want to use `use-auth-secret` and `static-auth-secret`, and have your signaling server generate temporary credentials.
* You might want to prevent coturn from relaying traffic to your internal network by using `denied-peer-ip` and `allowed-peer-ip`.
* Coturn issues:
  * [#699 Could not start Prometheus collector!](https://github.com/coturn/coturn/issues/699)
  * [#830 Bad configuration format: no-rfc5780](https://github.com/coturn/coturn/issues/830)

# References

* [coturn server](https://github.com/coturn/coturn)
  * [turnserver.conf](https://github.com/coturn/coturn/blob/docker/4.5.2-r3/examples/etc/turnserver.conf)
* [Configuring a Turn Server](https://matrix-org.github.io/synapse/develop/turn-howto.html)
* [Configuring coTURN](https://nextcloud-talk.readthedocs.io/en/turn_doc/TURN/)
* [WebRTC For The Curious](https://webrtcforthecurious.com/)
