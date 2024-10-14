# kubectl-like

`kubectl-like` is a tool for filtering Kubernetes Pod logs using regular expressions.

## Features

- Filter Pod logs using regular expressions
- Supports various Kubernetes resource types
- User-friendly CLI interface

## Installation

```sh
go install github.com/tae2089/kubectl-like@latest
```

## Usage

```sh
kubectl like (POD | TYPE/NAME) -p PATTERN [flags] [options]
```

## Example

To filter Pod logs using a specific pattern, run:

```sh
k like deployments/nginx --pattern 'error'
```

## Contributing

Contributions are welcome! To contribute, follow these steps:

Fork this repository.
Create a new branch (git checkout -b feature/your-feature).
Commit your changes (git commit -am 'Add some feature').
Push to the branch (git push origin feature/your-feature).
Create a Pull Request.

## Release

Releases are automatically generated using GitHub Actions. Push a tag to create a release:

## License

This project is licensed under the MIT License. See the LICENSE file for details.

```sh
This  file includes an overview of the project, installation instructions, usage examples, contribution guidelines, release instructions, and license information. You can modify or add content as needed.
```
