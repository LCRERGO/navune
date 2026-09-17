package com.example.gamma;

import com.example.alpha.Alpha;

public class Gamma {
    public static int compute(int x) {
        return new Alpha().run(x);
    }
}
