// Package pki contains trust-bootstrap scaffolding for Transitloom.
//
// Root and coordinator trust state stay explicit here because the root
// authority is a distinct trust role, not a normal node-facing coordinator.
// This package intentionally stops at local bootstrap/material checks for now;
// issuance flows, admission tokens, and control sessions belong to later tasks.
package pki
