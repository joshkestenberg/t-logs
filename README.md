# t-logs

Call the exe with 5 args:

1. filepath for log file (note: log file must be verbose ie. p2p msgs must be present)
2. filepath for peer file (see included peer file for format)
3. start line
4. end line
5. duration in milliseconds (min 2 max 59999) for each report statement

eg.

`./reader logs/tendermint-verbose.log peers 1 50000 30000`
