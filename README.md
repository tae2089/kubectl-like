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

## Shell completion

This plugin supports shell completion when used through kubectl. To enable shell completion for the plugin
you must copy the file `./kubectl_complete-like` somewhere on `$PATH` and give it executable permissions.

The `./kubectl_complete-like` script shows a hybrid approach to providing completions:

1. it uses the builtin `__complete` command provided by [Cobra](https://github.com/spf13/cobra) for flags
1. it calls `kubectl` to obtain the list of namespaces to complete arguments (note that a more elegant approach would be to have the `kubectl-like` program itself provide completion of arguments by implementing Cobra's `ValidArgsFunction` to fetch the list of namespaces, but it would then be a less varied example)

One can then do things like:

```
$ kubectl like <TAB>
daemonsets/                    deployments/                    jobs/                    pods/                    replicasets/
[...]

$ kubectl like --<TAB>
--all-containers                               -- Get all containers' logs in the pod(s).
--all-pods                                     -- Get logs from all pod(s). Sets prefix to true.
--as                                           -- Username to impersonate for the operation. User could be a regular user or a service account in a namespac
--as-group                                     -- Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
[...]
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
