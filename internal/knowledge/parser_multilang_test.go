package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

func TestParsePythonFile(t *testing.T) {
	src := `import os
from pathlib import Path

class DataProcessor:
    def __init__(self, path):
        self.path = path

    def process(self, data):
        return data.strip()

def main():
    dp = DataProcessor("test")
    dp.process("hello")
`
	path := writeTempFile(t, "processor.py", src)
	result, err := ParsePythonFile(path)
	if err != nil {
		t.Fatalf("ParsePythonFile: %v", err)
	}

	// Should find: module, class, __init__, process, main, 2 imports
	nodeNames := make(map[string]bool)
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}

	for _, want := range []string{"DataProcessor", "__init__", "process", "main", "os"} {
		if !nodeNames[want] {
			t.Errorf("missing node: %s", want)
		}
	}

	// __init__ and process should be methods
	for _, n := range result.Nodes {
		if n.Name == "__init__" && n.Kind != KindMethod {
			t.Errorf("__init__ should be KindMethod, got %v", n.Kind)
		}
		if n.Name == "main" && n.Kind != KindFunction {
			t.Errorf("main should be KindFunction, got %v", n.Kind)
		}
	}
}

func TestParseRustFile(t *testing.T) {
	src := `use std::io;
use std::collections::HashMap;

struct Config {
    name: String,
}

trait Processable {
    fn process(&self) -> String;
}

impl Processable for Config {
    fn process(&self) -> String {
        self.name.clone()
    }
}

pub fn main() {
    let c = Config { name: "test".to_string() };
    c.process();
}
`
	path := writeTempFile(t, "config.rs", src)
	result, err := ParseRustFile(path)
	if err != nil {
		t.Fatalf("ParseRustFile: %v", err)
	}

	nodeNames := make(map[string]bool)
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}

	for _, want := range []string{"Config", "Processable", "process", "main"} {
		if !nodeNames[want] {
			t.Errorf("missing node: %s", want)
		}
	}

	// Check for impl edge
	foundImpl := false
	for _, e := range result.Edges {
		if e.Kind == EdgeImplements && e.Target == "Processable" {
			foundImpl = true
		}
	}
	if !foundImpl {
		t.Error("missing impl Processable edge")
	}
}

func TestParseJSFile(t *testing.T) {
	src := `import express from 'express';
import { Router } from './router';

class Server {
    constructor(port) {
        this.port = port;
    }

    start() {
        console.log("starting");
    }
}

function createApp() {
    return new Server(3000);
}

const handler = (req, res) => {
    res.send("ok");
};

export default Server;
`
	path := writeTempFile(t, "server.js", src)
	result, err := ParseJSFile(path)
	if err != nil {
		t.Fatalf("ParseJSFile: %v", err)
	}

	nodeNames := make(map[string]bool)
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}

	for _, want := range []string{"Server", "start", "createApp", "handler"} {
		if !nodeNames[want] {
			t.Errorf("missing node: %s", want)
		}
	}

	// Imports
	foundExpress := false
	for _, n := range result.Nodes {
		if n.Kind == KindImport && n.Name == "express" {
			foundExpress = true
		}
	}
	if !foundExpress {
		t.Error("missing express import")
	}
}

func TestParseCFile(t *testing.T) {
	src := `#include <stdio.h>
#include "utils.h"

struct Point {
    int x;
    int y;
};

int add(int a, int b) {
    return a + b;
}

void print_point(struct Point p) {
    printf("%d, %d\n", p.x, p.y);
}
`
	path := writeTempFile(t, "math.c", src)
	result, err := ParseCFile(path)
	if err != nil {
		t.Fatalf("ParseCFile: %v", err)
	}

	nodeNames := make(map[string]bool)
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}

	for _, want := range []string{"Point", "add", "print_point"} {
		if !nodeNames[want] {
			t.Errorf("missing node: %s", want)
		}
	}

	// Includes
	foundStdio := false
	for _, n := range result.Nodes {
		if n.Kind == KindImport && n.Name == "stdio" {
			foundStdio = true
		}
	}
	if !foundStdio {
		t.Error("missing stdio include")
	}
}

func TestParseJavaFile(t *testing.T) {
	src := `import java.util.List;
import java.util.ArrayList;

public class UserService {
    public List<User> getUsers() {
        return new ArrayList<>();
    }

    private void validate(User user) {
        // validation logic
    }
}
`
	path := writeTempFile(t, "UserService.java", src)
	result, err := ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile: %v", err)
	}

	nodeNames := make(map[string]bool)
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}

	for _, want := range []string{"UserService", "getUsers", "validate"} {
		if !nodeNames[want] {
			t.Errorf("missing node: %s", want)
		}
	}

	// Class kind — check by ID (module.ClassName) to avoid matching the package node
	foundClass := false
	for _, n := range result.Nodes {
		if n.Name == "UserService" && n.Kind == KindStruct {
			foundClass = true
		}
	}
	if !foundClass {
		t.Error("UserService class node (KindStruct) not found")
	}
}

func TestParseTypeScriptFile(t *testing.T) {
	src := `import { Component } from '@angular/core';

export class AppComponent {
    title = 'my-app';

    onClick() {
        console.log('clicked');
    }
}

export function bootstrap() {
    console.log('bootstrapping');
}
`
	path := writeTempFile(t, "app.component.ts", src)
	result, err := ParseJSFile(path) // JS parser handles TS too
	if err != nil {
		t.Fatalf("ParseJSFile (TS): %v", err)
	}

	nodeNames := make(map[string]bool)
	for _, n := range result.Nodes {
		nodeNames[n.Name] = true
	}

	if !nodeNames["AppComponent"] {
		t.Error("missing AppComponent class")
	}
	if !nodeNames["bootstrap"] {
		t.Error("missing bootstrap function")
	}
}
