package com.example.alpha;

import com.example.beta.Beta;

public class Alpha {
    public int run(int x) {
        if (x > 0) {
            return Beta.value(x);
        }
        return 0;
    }
}

interface Marker {
    int marker();
}
