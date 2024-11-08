# HP LTO Max Temperature

# Description
This golang app provides functionality of getting max temperature of the LTO drive internals since boot or last cartrige mount as per HP specification.
That may be useful for understanding if your drive has good enough cooling.

*To get current temperature inside the drive instead use
```bash
sudo sg_logs -p temp  /dev/sg4
```

# Building
```bash
CGO_ENABLED=0 go build -v -a -ldflags '-extldflags "-static"' -o hp_lto_max_temp hp_lto_max_temp.go
```
# Installation
```bash
chmod +x hp_lto_max_temp
sudo mv hp_lto_max_temp /usr/local/bin/
hp_lto_max_temp --help
```

# Running
```bash
sudo hp_lto_max_temp /dev/sg0
```

# Reference
https://docs.oracle.com/cd/E38452_01/en/LTO6_Vol1_E1_D7/LTO6_Vol1_E1_D7.pdf 
![Screenshot from 2024-11-04 00-09-01](https://github.com/user-attachments/assets/6c7bab99-3f94-45a1-ac14-3071e7c36ec5)



