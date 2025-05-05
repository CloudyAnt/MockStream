# Mock Stream

This is a Fyne app, meant to mock stream http apis. Currently, this only overed the OpenAI style api `chat/completions`.

All other http requests will be proxied to designated URL if it presents.


## Building
```shell
fyne package --name "Mock Stream" --app-id com.cloudyant.mockstream
```