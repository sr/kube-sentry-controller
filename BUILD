load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/sr/kube-sentry-controller
gazelle(name = "gazelle")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "github.com/sr/kube-sentry-controller",
    visibility = ["//visibility:private"],
    deps = [
        "//pkg/apis:go_default_library",
        "//pkg/controller:go_default_library",
        "//pkg/sentry:go_default_library",
        "//vendor/github.com/pkg/errors:go_default_library",
        "//vendor/sigs.k8s.io/controller-runtime/pkg/client/config:go_default_library",
        "//vendor/sigs.k8s.io/controller-runtime/pkg/manager:go_default_library",
        "//vendor/sigs.k8s.io/controller-runtime/pkg/runtime/log:go_default_library",
        "//vendor/sigs.k8s.io/controller-runtime/pkg/runtime/signals:go_default_library",
    ],
)

go_binary(
    name = "project",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)

load("@io_bazel_rules_docker//go:image.bzl", "go_image")

go_image(
    name = "kube-sentry-controller",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
    goos = "linux",
    goarch = "amd64",
)
