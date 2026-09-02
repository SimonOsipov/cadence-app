import ComposeApp
import SwiftUI
import UIKit

/// ComposeView hands the whole screen to the Kotlin side. The Swift host owns
/// no UI of its own — everything a user sees is built once in :composeApp.
struct ComposeView: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> UIViewController {
        MainViewControllerKt.mainViewController()
    }

    func updateUIViewController(_ uiViewController: UIViewController, context: Context) {}
}

struct ContentView: View {
    var body: some View {
        ComposeView()
            .ignoresSafeArea()
            // Both the link the app was launched with and every one that arrives while it runs;
            // SwiftUI delivers them to the same place, and the Compose tree behind this outlives
            // all of them.
            .onOpenURL { MainViewControllerKt.openedWith(link: $0.absoluteString) }
    }
}
