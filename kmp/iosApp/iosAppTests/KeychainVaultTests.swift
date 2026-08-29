import ComposeApp
import XCTest

/// The Keychain suite, hosted by the app rather than run from `iosSimulatorArm64Test`.
///
/// Measured 2026-08-29: `SecItemAdd` from a Kotlin/Native test binary answers -25291,
/// `errSecNotAvailable` — that binary is not an app bundle and has no keychain to write to.
/// `KeychainReachabilityTest` in `shared/src/iosTest` pins that refusal; these are the
/// assertions it displaced, running where an app identity exists.
final class KeychainVaultTests: XCTestCase {
    private let service = "app.cadence.test-vault"
    private lazy var vault = KeychainVault(service: service)

    override func tearDown() {
        vault.wipe()
        super.tearDown()
    }

    func testWhatIsWrittenComesBack() {
        vault.write(bytes: bytes("rt-1"))

        XCTAssertEqual(text(vault.read()), "rt-1")
    }

    func testNothingWrittenReadsAsNothing() {
        XCTAssertNil(KeychainVault(service: "\(service).empty").read())
    }

    /// A second write updates rather than adding a second item: `SecItemAdd` over an existing
    /// account answers `errSecDuplicateItem`, and a vault ignoring that would keep handing back
    /// the session the patient signed out of.
    func testWritingTwiceUpdatesRatherThanDuplicates() {
        vault.write(bytes: bytes("rt-1"))
        vault.write(bytes: bytes("rt-2"))

        XCTAssertEqual(text(vault.read()), "rt-2")
    }

    func testWipingLeavesNothingBehind() {
        vault.write(bytes: bytes("rt-1"))

        vault.wipe()

        XCTAssertNil(vault.read())
    }

    /// Two services are two stores: the session and the PKCE verifier live side by side, and one
    /// clearing the other would sign the patient out in the middle of accepting an invite.
    func testTwoServicesDoNotShareAStore() {
        let other = KeychainVault(service: "\(service).other")
        defer { other.wipe() }
        vault.write(bytes: bytes("session"))
        other.write(bytes: bytes("verifier"))

        XCTAssertEqual(text(vault.read()), "session")
        XCTAssertEqual(text(other.read()), "verifier")
    }

    /// The two attributes that keep the session on this device, read back off the item itself.
    /// No behavioural assertion above can see them: a store written `WhenUnlocked` round-trips
    /// identically and still locks a background token refresh out, and one written
    /// synchronizable travels to the patient's other devices while reading the same here.
    func testTheItemIsReadableAfterFirstUnlockAndOnlyOnThisDevice() {
        vault.write(bytes: bytes("rt-1"))

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecReturnAttributes as String: true,
        ]
        var found: CFTypeRef?
        XCTAssertEqual(SecItemCopyMatching(query as CFDictionary, &found), errSecSuccess)

        let attributes = found as? [String: Any]
        XCTAssertEqual(
            attributes?[kSecAttrAccessible as String] as? String,
            kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String
        )
        XCTAssertEqual(attributes?[kSecAttrSynchronizable as String] as? Bool, false)
    }

    private func bytes(_ value: String) -> KotlinByteArray {
        let utf8 = Array(value.utf8)
        let array = KotlinByteArray(size: Int32(utf8.count))
        for (index, byte) in utf8.enumerated() {
            array.set(index: Int32(index), value: Int8(bitPattern: byte))
        }
        return array
    }

    private func text(_ array: KotlinByteArray?) -> String? {
        guard let array else { return nil }
        var utf8: [UInt8] = []
        for index in 0..<array.size {
            utf8.append(UInt8(bitPattern: array.get(index: index)))
        }
        return String(decoding: utf8, as: UTF8.self)
    }
}
