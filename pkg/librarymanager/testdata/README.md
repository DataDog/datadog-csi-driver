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

To recreate the test database, add and execute the following test:

```go
func TestCreateTestDatabase(t *testing.T) {
	basePath, err := filepath.Abs("testdata")
	require.NoError(t, err)

	path := filepath.Join(basePath, librarymanager.DatabaseFileName)
	err = os.Remove(path)
	require.NoError(t, err)

	db, err := librarymanager.NewDatabase(basePath)
	require.NoError(t, err)
	defer db.Close()

	volumeID := "test-volume-id"
	libraryID := "test-library-id"
	err = db.LinkVolume(libraryID, volumeID)
	require.NoError(t, err)
}
```
