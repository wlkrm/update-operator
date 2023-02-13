sudo ip link add macvlan1 link enp0s31f6 address a6:7b:b1:78:8a:61 type macvlan mode bridge 
sudo ip link add macvlan2 link enp0s31f6 address a6:7b:b1:78:8a:62 type macvlan mode bridge 
sudo ip link add macvlan3 link enp0s31f6 address a6:7b:b1:78:8a:63 type macvlan mode bridge
