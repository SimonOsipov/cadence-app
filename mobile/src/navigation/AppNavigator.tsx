// Navigation — the prototype's screen graph as a real native stack.
// Push screens slide from the right; log/add flows present as full-screen modals.
import React, { useState } from 'react';
import { View } from 'react-native';
import { NavigationContainer, DefaultTheme } from '@react-navigation/native';
import {
  createNativeStackNavigator,
  NativeStackNavigationProp,
} from '@react-navigation/native-stack';
import { pal } from '../theme';
import { useAppState } from '../state/AppState';
import { TabId } from '../components/shared';
import { ActionChooserSheet } from './ActionChooserSheet';
import { ConfirmToast } from './ConfirmToast';

import { TodayScreen } from '../features/today/TodayScreen';
import { LogDoseScreen } from '../features/log-dose/LogDoseScreen';
import { TrendsScreen } from '../features/trends/TrendsScreen';
import { TrendDetailScreen } from '../features/trends/TrendDetailScreen';
import { NutritionScreen } from '../features/meal/NutritionScreen';
import { LogMealScreen } from '../features/meal/LogMealScreen';
import { RecipesScreen } from '../features/recipe/RecipesScreen';
import { RecipeDetailScreen } from '../features/recipe/RecipeDetailScreen';
import { RecipeBuilderScreen } from '../features/recipe/RecipeBuilderScreen';
import { VialsScreen } from '../features/inventory/VialsScreen';
import { AddVialScreen } from '../features/inventory/AddVialScreen';
import { ScheduleScreen } from '../features/schedule/ScheduleScreen';
import { LearnScreen } from '../features/learn/LearnScreen';
import { ArticleScreen } from '../features/learn/ArticleScreen';
import { JournalScreen } from '../features/journal/JournalScreen';
import { BodyScreen } from '../features/body/BodyScreen';
import { ChatListScreen } from '../features/chat/ChatListScreen';
import { ChatThreadScreen } from '../features/chat/ChatThreadScreen';
import { ProfileScreen } from '../features/profile/ProfileScreen';

export type RootStackParamList = {
  Today: undefined;
  Trends: undefined;
  TrendDetail: { biomarkerId: string };
  Nutrition: undefined;
  Vials: undefined;
  Schedule: undefined;
  Learn: undefined;
  Article: { articleId: string };
  Journal: undefined;
  Body: undefined;
  Recipes: undefined;
  RecipeDetail: { recipeId: string };
  Profile: undefined;
  ChatList: undefined;
  ChatThread: { threadId: string };
  LogDose: undefined;
  LogMeal: undefined;
  AddVial: undefined;
  RecipeBuilder: undefined;
};

type Nav = NativeStackNavigationProp<RootStackParamList>;

const Stack = createNativeStackNavigator<RootStackParamList>();

const navTheme = {
  ...DefaultTheme,
  colors: { ...DefaultTheme.colors, background: pal.bg },
};

export function AppNavigator() {
  const app = useAppState();
  const [actionSheetOpen, setActionSheetOpen] = useState(false);
  const navRef = React.useRef<any>(null);

  // Tab bar handler shared by Today / Vials / Trends / Nutrition.
  const changeTab = (nav: Nav) => (id: TabId) => {
    if (id === 'log') {
      setActionSheetOpen(true);
      return;
    }
    if (id === 'today') {
      if (nav.canGoBack()) nav.popToTop();
      return;
    }
    const route = id === 'inventory' ? 'Vials' : id === 'insights' ? 'Trends' : 'Nutrition';
    if (nav.canGoBack()) nav.popToTop();
    nav.navigate(route as any);
  };

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <NavigationContainer ref={navRef} theme={navTheme}>
        <Stack.Navigator
          screenOptions={{
            headerShown: false,
            animation: 'slide_from_right',
            animationDuration: 380,
            contentStyle: { backgroundColor: pal.bg },
          }}
        >
          <Stack.Screen name="Today">
            {({ navigation }: { navigation: Nav }) => (
              <TodayScreen
                onLogDose={() => navigation.navigate('LogDose')}
                onPlusTap={() => setActionSheetOpen(true)}
                onOpenTrends={() => navigation.navigate('Trends')}
                onOpenTrend={(id: string) => navigation.navigate('TrendDetail', { biomarkerId: id })}
                onOpenVials={() => navigation.navigate('Vials')}
                onOpenChat={() => navigation.navigate('ChatThread', { threadId: 'ksenia' })}
                onOpenProfile={() => navigation.navigate('Profile')}
                onOpenSchedule={() => navigation.navigate('Schedule')}
                onOpenLearn={() => navigation.navigate('Learn')}
                onOpenJournal={() => navigation.navigate('Journal')}
                onLogMeal={() => navigation.navigate('LogMeal')}
                onOpenNutrition={() => navigation.navigate('Nutrition')}
                onOpenRecipes={() => navigation.navigate('Recipes')}
                onChangeTab={changeTab(navigation)}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Trends">
            {({ navigation }: { navigation: Nav }) => (
              <TrendsScreen
                onBack={() => navigation.goBack()}
                onOpenDetail={(id: string) => navigation.navigate('TrendDetail', { biomarkerId: id })}
                onOpenJournal={() => navigation.navigate('Journal')}
                onOpenBody={() => navigation.navigate('Body')}
                onChangeTab={changeTab(navigation)}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="TrendDetail">
            {({ navigation, route }: { navigation: Nav; route: any }) => (
              <TrendDetailScreen
                biomarkerId={route.params.biomarkerId}
                onBack={() => navigation.goBack()}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Nutrition">
            {({ navigation }: { navigation: Nav }) => (
              <NutritionScreen
                onBack={() => navigation.goBack()}
                onLogMeal={() => navigation.navigate('LogMeal')}
                onOpenRecipes={() => navigation.navigate('Recipes')}
                onChangeTab={changeTab(navigation)}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Vials">
            {({ navigation }: { navigation: Nav }) => (
              <VialsScreen
                onBack={() => navigation.goBack()}
                onAddVial={() => navigation.navigate('AddVial')}
                onLogDose={() => navigation.navigate('LogDose')}
                onChangeTab={changeTab(navigation)}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Schedule">
            {({ navigation }: { navigation: Nav }) => (
              <ScheduleScreen
                onBack={() => navigation.goBack()}
                onLogDose={() => navigation.navigate('LogDose')}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Learn">
            {({ navigation }: { navigation: Nav }) => (
              <LearnScreen
                onBack={() => navigation.goBack()}
                onOpenArticle={(id: string) => navigation.navigate('Article', { articleId: id })}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Article">
            {({ navigation, route }: { navigation: Nav; route: any }) => (
              <ArticleScreen
                articleId={route.params.articleId}
                onBack={() => navigation.goBack()}
                onOpenArticle={(id: string) => navigation.push('Article', { articleId: id })}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Journal">
            {({ navigation }: { navigation: Nav }) => (
              <JournalScreen onBack={() => navigation.goBack()} />
            )}
          </Stack.Screen>

          <Stack.Screen name="Body">
            {({ navigation }: { navigation: Nav }) => (
              <BodyScreen
                onBack={() => navigation.goBack()}
                onOpenTrend={(id: string) => navigation.navigate('TrendDetail', { biomarkerId: id })}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Recipes">
            {({ navigation }: { navigation: Nav }) => (
              <RecipesScreen
                onBack={() => navigation.goBack()}
                onOpen={(id: string) => navigation.navigate('RecipeDetail', { recipeId: id })}
                onCreate={() => navigation.navigate('RecipeBuilder')}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="RecipeDetail">
            {({ navigation, route }: { navigation: Nav; route: any }) => (
              <RecipeDetailScreen
                recipeId={route.params.recipeId}
                onBack={() => navigation.goBack()}
                onAddToDay={(recipe: any, portions: number) => {
                  app.addRecipeToDay(recipe, portions);
                  navigation.navigate('Nutrition');
                }}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="Profile">
            {({ navigation }: { navigation: Nav }) => (
              <ProfileScreen
                onBack={() => navigation.goBack()}
                onOpenChat={(threadId?: string) =>
                  navigation.navigate('ChatThread', { threadId: threadId || 'ksenia' })
                }
                onOpenTrend={(id: string) => navigation.navigate('TrendDetail', { biomarkerId: id })}
                onOpenSchedule={() => navigation.navigate('Schedule')}
                onOpenJournal={() => navigation.navigate('Journal')}
                onOpenBody={() => navigation.navigate('Body')}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="ChatList">
            {({ navigation }: { navigation: Nav }) => (
              <ChatListScreen
                onBack={() => navigation.goBack()}
                onOpenThread={(id: string) => navigation.navigate('ChatThread', { threadId: id })}
              />
            )}
          </Stack.Screen>

          <Stack.Screen name="ChatThread">
            {({ navigation, route }: { navigation: Nav; route: any }) => (
              <ChatThreadScreen
                threadId={route.params.threadId}
                onBack={() => navigation.goBack()}
                onBackToList={() => navigation.replace('ChatList')}
              />
            )}
          </Stack.Screen>

          <Stack.Group
            screenOptions={{ presentation: 'fullScreenModal', animation: 'slide_from_bottom' }}
          >
            <Stack.Screen name="LogDose">
              {({ navigation }: { navigation: Nav }) => (
                <LogDoseScreen
                  onCancel={() => navigation.goBack()}
                  onComplete={(site?: string) => {
                    app.completeDoseLog(site);
                    navigation.goBack();
                  }}
                />
              )}
            </Stack.Screen>

            <Stack.Screen name="LogMeal">
              {({ navigation }: { navigation: Nav }) => (
                <LogMealScreen
                  onCancel={() => navigation.goBack()}
                  onComplete={(meal: any) => {
                    app.logMeal(meal);
                    navigation.goBack();
                  }}
                />
              )}
            </Stack.Screen>

            <Stack.Screen name="AddVial">
              {({ navigation }: { navigation: Nav }) => (
                <AddVialScreen
                  onCancel={() => navigation.goBack()}
                  onComplete={() => navigation.goBack()}
                />
              )}
            </Stack.Screen>

            <Stack.Screen name="RecipeBuilder">
              {({ navigation }: { navigation: Nav }) => (
                <RecipeBuilderScreen
                  onCancel={() => navigation.goBack()}
                  onSave={(recipe: any) => {
                    app.saveRecipe(recipe);
                    navigation.replace('RecipeDetail', { recipeId: recipe.id });
                  }}
                />
              )}
            </Stack.Screen>
          </Stack.Group>
        </Stack.Navigator>
      </NavigationContainer>

      <ActionChooserSheet
        open={actionSheetOpen}
        onClose={() => setActionSheetOpen(false)}
        onPickDose={() => {
          setActionSheetOpen(false);
          navRef.current?.navigate('LogDose');
        }}
        onPickMeal={() => {
          setActionSheetOpen(false);
          navRef.current?.navigate('LogMeal');
        }}
      />
      <ConfirmToast />
    </View>
  );
}
