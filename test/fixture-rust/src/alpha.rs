use crate::beta::value;

pub fn run(x: i32) -> i32 {
    if x > 0 {
        value(x)
    } else {
        0
    }
}

pub trait Marker {
    fn marker(&self) -> i32;
}
