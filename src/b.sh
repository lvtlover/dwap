
#!/usr/bin/bash

output_name=$1

src_path="cmd/zipsv/main.go"

go build -o $output_name $src_path
 