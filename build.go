package main

// This will build both the frontend and the server itself
// the built web page is embeded into the executable

//go:generate npm install --prefix ./frontend
//go:generate npm run build --prefix ./frontend

//go:generate go build .
