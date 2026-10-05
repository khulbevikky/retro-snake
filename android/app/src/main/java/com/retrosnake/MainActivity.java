package com.retrosnake;

import android.app.Activity;
import android.os.Bundle;
import com.retrosnake.mobile.EbitenView;

public class MainActivity extends Activity {
    private EbitenView gameView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        gameView = new EbitenView(this);
        setContentView(gameView);
    }

    @Override
    protected void onPause() {
        super.onPause();
        if (gameView != null) {
            gameView.suspendGame();
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (gameView != null) {
            gameView.resumeGame();
        }
    }
}

