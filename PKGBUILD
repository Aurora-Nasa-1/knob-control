# Maintainer: AuroraNasa
pkgname=volume-knob-control
pkgver=2.0.0
pkgrel=1
pkgdesc="A lightweight volume knob controller with device switching and brightness control."
arch=('x86_64' 'aarch64')
url="https://github.com/Aurora-Nasa-1/volume-control"
license=('MIT')
depends=('pipewire' 'libevdev' 'ddcutil')
makedepends=('go')
source=("$pkgname-$pkgver.tar.gz::$url/archive/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
    cd "$pkgname-$pkgver"
    export CGO_CPPFLAGS="${CPPFLAGS}"
    export CGO_CFLAGS="${CFLAGS}"
    export CGO_CXXFLAGS="${CXXFLAGS}"
    export CGO_LDFLAGS="${LDFLAGS}"
    export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
    go build -o volume-knob-control .
}

package() {
    cd "$pkgname-$pkgver"
    install -Dm755 volume-knob-control "${pkgdir}/usr/bin/volume-knob-control"
    install -Dm644 LICENSE "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
    install -Dm644 README.md "${pkgdir}/usr/share/doc/${pkgname}/README.md"
}
