# Instructions

To recreated the rootfs tarball:

```bash
tar --disable-copyfile -cvf rootfs.tar contents
```

To recreate the image tarball:

```bash
docker build -t  data .
docker save data -o image.tar
```
