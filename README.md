## Iperf-Gui

### 1. Dependancies
- Verified with iperf 3.6 which support `--forceflush` option on Ubuntu 18.04
- Verified with iperf 2.0.10 on Ubuntu 18.04

### 2. Install Latest Iperf3 on Ubuntu 18.04
```
# Switch to root
apt-get remove iperf3 libiperf0
wget https://downloads.es.net/pub/iperf/iperf-3.6.tar.gz
tar xvf iperf-3.6.tar.gz
rm -rf iperf-3.6.tar.gz
cd iperf-3.6/
apt-get install libtool m4 automake
./bootstrap.sh
./configure
make
make install

# Fix issue https://github.com/esnet/iperf/issues/153
ldconfig

iperf3 -v
```
