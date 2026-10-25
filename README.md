# docker_mini

Tự viết lại một Docker rút gọn bằng Go, để hiểu container hoạt động từ bên trong: namespace, cgroups, overlayfs, image, daemon và mạng.

Mỗi bước là một phần nhỏ, có cách kiểm chứng. Xem [ROADMAP.md](ROADMAP.md) để biết lộ trình.

## Yêu cầu

- Linux. Namespace, cgroups và overlayfs là tính năng của kernel Linux.
- Go (phiên bản trong `go.mod`).
- Quyền root (`sudo`) cho các bước mount và namespace.
