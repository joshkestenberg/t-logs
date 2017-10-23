# t-logs

This CLI is intended as a debugging tool for a network of Tendermint nodes.

 Using the ```state``` command, you'll be able to view the consensus state from the perspective of a given node at a given time.

 Using the ```msgs``` command, you'll be able to see from which peers a given node has received messages over the course of a given time span, at given intervals.

 ---

 To configue the program, use the ```nodes``` command to produce parseable log files, as well as an ip:pubkey json value store.  The ```nodes``` command takes as arguments the names of all (note: you must include the logs for **all** nodes in the P2P network).

 eg.

 ```t-logs nodes $HOME/node1.log $HOME/node2.log $HOME/node3.log```

 This should produce one file prefixed with "rendered_" for each given log file, and a file entitled "nodes.json", which functions as the aforementioned value store.

 ---

 For each call of either ```msgs``` or ```state```, a rendered log file must be provided as an argument to the flag ```--log``` so to establish a node from whose prespective we'll be looking at things.

 eg.

 ```t-logs --log ./rendered_node1.log state 08-14 04:33:39.426```

 which will provide us with the consensus state from the perspective of node1 at the given date and time.

 ---

 For further instructions on use of ```state``` and ```msgs```, please utilize the ```--help``` commad on each.
