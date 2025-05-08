# Mock Stream

This is a Fyne app, meant to mock stream http apis. Currently, it only covered the OpenAI style api `chat/completions`.

All other http requests will be proxied to designated URL if it presents.


## Packaging 

make sure the `fyne` command has been installed:

```shell
go install fyne.io/tools/cmd/fyne@latest
```

then bundle the icon:

```shell
fyne bundle -o bundled.go assets/icon.png
```

package:

```shell
fyne package --name "Mock Stream" --app-id com.cloudyant.mockstream
```
