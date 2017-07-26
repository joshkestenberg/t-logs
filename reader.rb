# require 'ffi'
#
# module Reader
#   extend FFI::Library
#   ffi_lib './reader.so'
#
#   attach_function :reader, [:string, :string], :string
#
# end
#
# print Reader.reader("tendermint.log", "tendermint.json")

`./reader tendermint.log log.json 10000 20000`
