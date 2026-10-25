# Lộ trình docker_mini

Mục tiêu: hiểu Docker từ bên trong bằng cách tự viết lại phần lõi của nó bằng Go.
Không làm lại toàn bộ Docker. Chỉ làm phần tạo nên container: cô lập tiến trình, image, daemon, mạng, volume và build.

Quy ước:
- Mỗi bước là một thay đổi nhỏ, chạy được, và có cách kiểm chứng rõ ràng.
- Các bước mount và namespace cần quyền root (`sudo`).
- Rootfs thử nghiệm lấy từ `alpine` minirootfs, đặt trong thư mục tạm (`/tmp/...`), không dùng `/`.
- Mọi `mount` phải chạy trong mount namespace riêng, đặt propagation thành `private` trước, để không rò ra host.
- Không dùng `runc`, `containerd` hay Docker thật làm nền. Chỉ dùng syscall của Linux qua `golang.org/x/sys/unix`.

| # | Bước | Kĩ thuật Docker / Linux | Go | Kiểm chứng | Trạng thái |
|---|------|-------------------------|----|------------|------------|
| 1 | CLI + chạy tiến trình con | Chưa cô lập. Container lúc này chỉ là một tiến trình | `os/exec`, `os.Args`, `flag`, nối stdin/stdout | `go run . run /bin/echo hi` | |
| 2 | UTS namespace | Cô lập hostname. Tái khởi động chính binary qua `/proc/self/exe` làm init | `syscall.SysProcAttr{Cloneflags}`, build tag `linux` | `hostname` trong container khác host | |
| 3 | PID + mount namespace | Process đầu tiên trong container là PID 1. Mount `/proc` riêng | `syscall.Mount`, `MS_PRIVATE`, `MS_REC` | `ps` trong container chỉ thấy vài process | |
| 4 | Rootfs bằng `pivot_root` | Cô lập filesystem: đổi root sang thư mục rootfs | `syscall.PivotRoot`, `os.Chdir`, `os.MkdirTemp` | `ls /` trong container chỉ thấy rootfs | |
| 5 | cgroups v2 | Giới hạn memory, CPU, số process qua `/sys/fs/cgroup` (`memory.max`, `cpu.max`, `pids.max`) | ghi file, `strconv`, `defer` để dọn dẹp | `pids.max=20`: fork quá 20 process thì bị từ chối | |
| 6 | Signal, exit code, zombie | Init nhận và chuyển tiếp signal, thu dọn tiến trình con (reap) | `os/signal`, `context`, `errors.As` với `*exec.ExitError` | Ctrl+C dừng container. `echo $?` ra đúng exit code | |
| 7 | Import rootfs từ tar.gz | Image ở mức thấp nhất chỉ là một cây thư mục nén | `archive/tar`, `compress/gzip`, `io.Copy` | Giải nén alpine minirootfs rồi chạy được | |
| 8 | Content-addressable store | Blob đặt tên theo sha256 (`blobs/sha256/<hex>`). Trùng nội dung thì không ghi lại | `crypto/sha256`, `io.TeeReader`, `os.Rename` | `sha256sum` khớp tên file | |
| 9 | Layer + overlayfs | `lowerdir` chỉ đọc xếp chồng, `upperdir` là lớp ghi của container (copy-on-write) | `filepath.Join`, `syscall.Mount("overlay", ...)` | Ghi file trong container, thư mục lower không đổi | |
| 10 | Manifest + config (OCI image spec) | Image = manifest trỏ tới config (ENV, CMD, ENTRYPOINT, WORKDIR) và danh sách layer | `encoding/json`, struct tag, `omitempty` | Parse được file JSON mẫu theo OCI spec | |
| 11 | Pull từ registry | OCI Distribution API: token auth, `GET /v2/<name>/manifests/<tag>`, tải blob theo digest | `net/http`, `context`, `fmt.Errorf` với `%w` | `docker_mini pull alpine` rồi `run` được | |
| 12 | Trạng thái container trên đĩa | Mỗi container có ID, PID, trạng thái. Lưu `containers/<id>/state.json` | `crypto/rand` (ID), `os.MkdirAll`, `json.Marshal` | Lệnh `ps` liệt kê đúng container đang có | |
| 13 | Lệnh `stop`, `rm`, `logs` | Gửi `SIGTERM` tới PID. Ghi stdout ra file log | `os.FindProcess`, `syscall.Kill`, `bufio` | `logs` thấy output của container | |
| 14 | Daemon + API qua Unix socket | `dockerd` nhận REST API trên socket. CLI chỉ là client | `net.Listen("unix")`, `net/http`, `http.Transport.DialContext` | `curl --unix-socket ... /containers/json` | |
| 15 | Chạy nền `-d` và `exec` | `setns` vào namespace của container đang chạy | `golang.org/x/sys/unix` (`Setns`), `runtime.LockOSThread` | `exec` vào container thấy đúng hostname | |
| 16 | Network namespace + veth + bridge | Mỗi container có netns riêng. Cặp veth nối vào bridge `dmini0` | netlink (`vishvananda/netlink` hoặc tự viết socket netlink) | Container ping được gateway của bridge | |
| 17 | IPAM + NAT | Cấp IP từ subnet. `MASQUERADE` để ra internet | `net/netip`, `os/exec` gọi `nft`/`iptables` | `wget` từ container ra được internet | |
| 18 | Port mapping `-p host:container` | DNAT để chuyển traffic từ host vào container | `strconv`, `net.SplitHostPort` | `curl localhost:<port>` tới được container | |
| 19 | Bind mount `-v host:container` | `mount --bind` thư mục host vào rootfs | `os.Stat`, `filepath.Clean` | File ghi từ container xuất hiện trên host | |
| 20 | Dockerfile parser | Cú pháp: `FROM`, `RUN`, `COPY`, `ENV`, `WORKDIR`, `CMD` | `bufio.Scanner`, `strings`, `switch`, `struct` | Parse Dockerfile mẫu thành danh sách lệnh | |
| 21 | Build: mỗi lệnh là một layer, có cache | Cache key = hash(layer cha + lệnh). Build lại thì dùng cache | `sync`, `errgroup` (nếu cần), `crypto/sha256` | Build lần 2 không chạy lại lệnh nào | |

Bonus (làm sau khi xong bước 21):
- User namespace (rootless)
- Bỏ bớt capabilities và seccomp filter (`golang.org/x/sys/unix`, `prctl`)
- Dọn dẹp layer không dùng (giống `docker system prune`)

Ngoài phạm vi: Swarm, Compose, registry server, BuildKit, Windows và macOS.

## Chạy thử

```sh
go test ./...                    # chạy test
go build -o docker_mini .        # build binary
sudo ./docker_mini run /bin/sh   # chạy lệnh (từ bước 1 trở đi)
```
