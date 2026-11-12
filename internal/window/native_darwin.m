#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#include <stdlib.h>
#include <string.h>
#include "native_darwin.h"

char *ks_window_list(void) {
 @autoreleasepool {
  NSMutableArray *out=[NSMutableArray array];
  NSArray *wins=CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionAll|kCGWindowListExcludeDesktopElements,kCGNullWindowID));
  for(NSDictionary *w in [wins reverseObjectEnumerator]) {
   if([w[(id)kCGWindowLayer] intValue]!=0) continue;
   int pid=[w[(id)kCGWindowOwnerPID] intValue];
   NSRunningApplication *a=[NSRunningApplication runningApplicationWithProcessIdentifier:pid];
   if(!a||a.activationPolicy!=NSApplicationActivationPolicyRegular) continue;
   NSString *bundle=a.bundleIdentifier?:@"";
   NSString *identifier=[NSString stringWithFormat:@"%d:%@",pid,w[(id)kCGWindowNumber]];
   [out addObject:@{@"id":identifier,@"pid":@(pid),@"class":bundle,@"app_id":bundle,@"active":@(a.active)}];
  }
  NSData *json=[NSJSONSerialization dataWithJSONObject:out options:0 error:nil];
  return strdup([[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding].UTF8String);
 }
}
int ks_window_activate(int pid) {
 @autoreleasepool {
  if(!AXIsProcessTrusted()) return -2;
  NSRunningApplication *a=[NSRunningApplication runningApplicationWithProcessIdentifier:pid];if(!a) return -1;
  AXUIElementRef app=AXUIElementCreateApplication(pid);CFTypeRef windows=NULL;
  if(AXUIElementCopyAttributeValue(app,kAXWindowsAttribute,&windows)==kAXErrorSuccess&&windows){
   CFArrayRef list=(CFArrayRef)windows;
   for(CFIndex i=0;i<CFArrayGetCount(list);i++){AXUIElementRef win=(AXUIElementRef)CFArrayGetValueAtIndex(list,i);AXUIElementSetAttributeValue(win,kAXMinimizedAttribute,kCFBooleanFalse);AXUIElementPerformAction(win,kAXRaiseAction);}
   CFRelease(windows);
  }
  CFRelease(app);
  return [a activateWithOptions:NSApplicationActivateAllWindows|NSApplicationActivateIgnoringOtherApps]?0:-1;
 }
}
