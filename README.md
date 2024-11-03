# hp-lto-max-temperature

# Building
CGO_ENABLED=0 go build -v -a -ldflags '-extldflags "-static"' -o hp_lto_max_temp hp_lto_max_temp.go

# Running
sudo ./hp_lto_max_temp /dev/sg0

# Reference
https://docs.oracle.com/cd/E38452_01/en/LTO6_Vol1_E1_D7/LTO6_Vol1_E1_D7.pdf 
