package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon -framework ApplicationServices

#import <Carbon/Carbon.h>
#import <ApplicationServices/ApplicationServices.h>

static CGEventRef eventCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
    if (type == kCGEventKeyDown) {
        CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
        CGEventFlags flags = CGEventGetFlags(event);
        
        // Option+Shift+I (keycode 34 = i)
        if (keyCode == 34 && 
            (flags & kCGEventFlagMaskAlternate) &&
            (flags & kCGEventFlagMaskShift)) {
            
            // Call Go function
            extern void launchGUI();
            launchGUI();
            
            // Consume the event
            return NULL;
        }
    }
    return event;
}

static int setupHotkey() {
    // Create event tap
    CGEventMask eventMask = CGEventMaskBit(kCGEventKeyDown);
    CFMachPortRef eventTap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionDefault,
        eventMask,
        eventCallback,
        NULL
    );
    
    if (!eventTap) {
        return -1;
    }
    
    // Create run loop source
    CFRunLoopSourceRef runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, eventTap, 0);
    CFRunLoopAddSource(CFRunLoopGetCurrent(), runLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(eventTap, true);
    
    return 0;
}

static void runEventLoop() {
    CFRunLoopRun();
}
*/
import "C"

var guiPath string

//export launchGUI
func launchGUI() {
	fmt.Println("Hotkey pressed! Launching GUI...")
	cmd := exec.Command("open", guiPath)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch GUI: %v\n", err)
	}
}

func findGUIApp() string {
	// Check common locations
	locations := []string{
		filepath.Join(os.Getenv("HOME"), "code/imgupv2/gui/build/bin/imgupv2-gui.app"),
		"/Applications/imgupv2-gui.app",
		filepath.Join(os.Getenv("HOME"), "Applications/imgupv2-gui.app"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

func main() {
	// Find the GUI app
	guiPath = findGUIApp()
	if guiPath == "" {
		log.Fatal("Could not find imgupv2-gui app")
	}

	fmt.Println("imgupv2 hotkey daemon starting...")
	fmt.Println("GUI found at:", guiPath)
	
	// Setup the hotkey
	if C.setupHotkey() != 0 {
		fmt.Println("\n⚠️  PERMISSION REQUIRED:")
		fmt.Println("Please grant accessibility access to this app:")
		fmt.Println("System Preferences > Security & Privacy > Privacy > Accessibility")
		fmt.Println("\nThen restart this app.")
		
		// Keep trying every few seconds
		for {
			time.Sleep(3 * time.Second)
			if C.setupHotkey() == 0 {
				fmt.Println("\n✅ Permission granted! Hotkey is now active.")
				break
			}
		}
	}
	
	fmt.Println("\n✅ Hotkey registered: Option+Shift+I")
	fmt.Println("Press Ctrl+C to quit")
	
	// Run the event loop
	C.runEventLoop()
}
