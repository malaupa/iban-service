#/usr/bin/env bash
CONFIGURATIONS=(
    windows,386 \
    windows,amd64 \
    linux,386 \
    linux,amd64 \
    linux,arm \
    linux,arm64
)

version=`cat VERSION`

for config in ${CONFIGURATIONS[@]}; do 
IFS=","
set $config

os="$1"
arch="$2"
base_path="build/$os/$arch"
path="$base_path/iban-service"
mkdir -p "$path"
bin_name="iban-service"

if [ $os = "windows" ]; then
    bin_name="$bin_name.exe"
fi

GOOS="$os" GOARCH="$arch" go build -ldflags "-X main.Version=${version}" -o "$path/$bin_name"
cp -r data "$path/"
tar czvf "build/iban-service-$version-$os-$arch.tar.gz" -C "$base_path" iban-service

unset IFS;
done