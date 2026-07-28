import ComposeApp
import SwiftUI
import UIKit

/// ComposeView hands the whole screen to the Kotlin side. The Swift host owns
/// no UI of its own — everything a user sees is built once in :composeApp.
struct ComposeView: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> UIViewController {
        MainViewControllerKt.MainViewController()
    }

    func updateUIViewController(_ uiViewController: UIViewController, context: Context) {}
}

struct ContentView: View {
    var body: some View {
        ComposeView()
            .ignoresSafeArea()
    }
}
