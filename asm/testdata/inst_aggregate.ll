define void @f() {
	extractvalue { i8, { i32, i64 } } { i8 1, { i32, i64 } { i32 2, i64 3 } }, 1, 1
	insertvalue { i8, { i32, i64 } } { i8 1, { i32, i64 } { i32 2, i64 3 } }, i64 4, 1, 1
	ret void
}
